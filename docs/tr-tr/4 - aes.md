# AES: kalıcı retry ve kayıt temizleme

AES tap olaylarını veritabanına kaydeder; request/response logları varsayılan
olarak kaydedilmez, yalnızca `request_rules` ile seçilenler saklanır.
Kuralla eşleşen event teslimatları,
mesaj + subject + tüketici grubu bazında ayrıca takip edilir. Bir gruptaki hata
diğer grupların başarılı teslimatlarını etkilemez.

[English](../en-us/4%20-%20aes.md)

## Yapılandırma

`aes.env` yanında `aes.rules.json` bulunur. Dosya yolu çalışma dizinine göre
çözülür; farklı bir konum için `E5_AES_RULES_FILE` kullanın. Env dosyasını
çalıştırıcı/IDE ortam değişkenlerine yüklemelidir; AES kendisi env dosyası okumaz.

Boş bir `rules` listesi event loglamasını sürdürür, otomatik event retry ve
temizlemeyi kapatır. `request_rules` eksik veya boşsa yeni request/response
logları kaydedilmez. Örnek event kuralları:

```json
{
  "rules": [
    {
      "event_subject_key": "order.created",
      "instance_group": "^orders$",
      "instance_name": "*",
      "try_after": 30,
      "try_count": 5,
      "consumption_timeout": 120,
      "on_consumption_timeout": "try",
      "remove_successful_timeout": 3600,
      "remove_failed_timeout": 86400
    },
    {
      "event_subject_key": { "regex": "^payment\\." }
    }
  ]
}
```

| Alan | Varsayılan | Anlamı |
|---|---|---|
| `event_subject_key` | Zorunlu | String: birebir subject; `{"regex":"..."}`: Go regex |
| `instance_group` | `"*"` | Kuralın uygulanacağı tüketici grubu; `"*"` veya regex |
| `instance_name` | `"*"` | Kuralın uygulanacağı ilk tüketicinin ismi; `"*"` veya regex |
| `try_after` | `0` | Hata/timeout sonrası bekleme, saniye; `0`: retry yok |
| `try_count` | `0` | İlk gönderim hariç retry sayısı; `0`: sınırsız |
| `consumption_timeout` | `0` | Teslimat sonrası sonuç bekleme, saniye; `0`: süre sınırı yok |
| `on_consumption_timeout` | `"keep"` | Süre dolunca `keep`, `try` veya `remove` |
| `remove_successful_timeout` | `3600` | Başarıdan sonra saklama, saniye; `0`: silme |
| `remove_failed_timeout` | `0` | Terminal başarısızlıktan sonra saklama, saniye; `0`: silme |

Birden fazla eşleşmede ilk kural kazanır. Regex için tüm değeri eşleştirmek
istiyorsanız `^...$` kullanın. `"*"` özel bir tümünü eşleştirme değeridir, regex
olarak derlenmez. Negatif süre/sayı, geçersiz regex ve bilinmeyen alanlar
başlangıçta hata verir. Kurallar başlangıçta okunur; hot reload yoktur.

Event kuralıyla eşleşmeyen event teslimatları yalnızca loglanır, otomatik temizlenmez.
Her takip edilen teslimatta kural değerleri saklanır; kuralları değiştirmek veya
AES'i yeniden başlatmak bekleyen teslimatın sayacını/deadline'ını sıfırlamaz.

### Consumption timeout

- `keep`: Kayıt korunur; otomatik retry/temizleme yapılmaz. Sonradan gelen
  consumption sonucu yine uygulanır.
- `try`: Timeout tüketim hatası sayılır. `try_after` ve `try_count` uygulanır.
  Retry kapalıysa/limit tükenmişse başarısız saklama süresi uygulanır.
- `remove`: Süre dolunca kayıt ve ilişkili logları silinir; retry yapılmaz.
  Bu açık bir veri kaybı tercihidir, varsayılan değildir.

`consumption_timeout=0` olduğunda action ne olursa olsun timeout çalışmaz.
Son retry gönderilmesi tek başına başarısızlık değildir; sonuç veya
yapılandırılmış timeout beklenir. Başarıda limit dolmasa bile retry durur ve
başarılı saklama süresi başlar. Bekleyen/sınırsız retry kayıtları başarısız
saklama süresi doldu diye silinmez.

## Request/response loglarının saklanması

Event kurallarından bağımsız, isteğe bağlı bir `request_rules` listesi kullanılır:

```json
{
  "rules": [],
  "request_rules": [
    {
      "request_subject_key": { "regex": "^postral/important/" },
      "remove_timeout": 30
    },
    {
      "request_subject_key": "audit/payment-details",
      "remove_timeout": 0
    }
  ]
}
```

| Alan | Varsayılan | Anlamı |
|---|---|---|
| `request_subject_key` | Zorunlu | Birebir subject string'i veya `{"regex":"..."}` |
| `remove_timeout` | `3600` | İlk gözlenen request logundan itibaren saklama süresi, saniye; `0`: süresiz sakla |

İlk eşleşen request kuralı kazanır. Grup/instance filtresi veya retry alanı
yoktur; request'ler tekrar gönderilmez. Saklama kararı tüm request/response
akışına aynı `messageId` üzerinden uygulanır:
`REQUEST_RECEIVED`, `REQUEST_ROUTED`, `REQUEST_NO_TARGET`,
`RESPONSE_RECEIVED`, `RESPONSE_DELIVERED`.

**Varsayılan kaydetmemektir.** Eşleşmeyen request'lerin body'leri DB'ye hiç
yazılmaz. `request_rules` listesini eklememek veya boş bırakmak bu davranışı
etkinleştirir; event kuralları request'lere uygulanmaz.

Eşleşen request'in subject'i, saklama süresi ve deadline'ı `aes_request`
tablosunda tutulur. Subject'i boş gelen response, bu kayıttan tamamlanır.
Yeni bir response, tek başına saklama kaydı başlatmaz; request'i gözlenmemiş
veya temizlenmiş response'lar kaydedilmez.

Pozitif süre **cevaptan değil ilk request logunun AES tarafından alınmasından**
başlar; sonraki loglar süreyi uzatmaz. Cevap/hata gelmese bile süresi dolan
request'in tüm ilişkili request/response logları ve metadata'sı temizlenir.
Süre dolduğu anda gelen response kaydedilmez; geç gelen response silinmiş
request'i yeniden açmaz. `remove_timeout=0` kayıtları süresiz tutar; varsayılan
"kaydetme" ile aynı anlamda değildir.

Kurallar başlangıçta okunur. AES'i yeniden başlatın; önceden seçilmiş request'ler
kendi kayıtlı süreleriyle devam eder. Önceki sürümden kalan, saklama metadata'sı
olmayan request/response logları **otomatik veya geriye dönük silinmez**.

## Bağlantılar ve veritabanı

AES iki ayrı bağlantı kullanır:

- Tap: `E5_EXHAUST_HOST`, `E5_EXHAUST_PORT`, `E5_EXHAUST_KEY`,
  `E5_EXHAUST_TLS`, `E5_EXHAUST_CA_FILE`, `E5_EXHAUST_RECONNECT`.
- Retry yayını: `E5_HOST` (localhost), `E5_PORT` (3535), `E5_TLS` (false),
  `E5_AES_AUTH_KEY` (boş). Sertifika CA dosyası `E5_EXHAUST_CA_FILE` ile ortaktır.
  Broker auth açıksa AES kimliğinin publish ve ilgili subject yetkisi olmalıdır.

TLS ayarları ilgili sunucu portuyla uyumlu olmalıdır; doğrulama atlanmaz.
Tap'a orijinal body gider. `E5_EXHAUST_INCLUDE_CONTENT` **yalnızca konsol**
maskelemesini kontrol eder; AES body saklaması bu değişkene bağlı değildir.

İlk şema MySQL içindir. `E5_AES_DB_DRIVER=mysql`, port varsayılanı `3306`;
diğer `E5_AES_DB_*` bağlantı ayarları korunur. Önceki PostgreSQL varsayılanı
çalışır bir PostgreSQL driver/DSN uygulamasına sahip olmadığından kaldırılmıştır.
SQLite yalnızca izole otomatik testlerde kullanılır.

`E5_AES_DB_GENERATE_IF_NOT_EXIST=true` şu tabloları oluşturur:

- `aes_event_tap`: mevcut olay logları.
- `aes_delivery`: body, durum, retry sayacı, zamanlar ve policy snapshot.
- `aes_attempt`: attempt message ID → delivery eşleştirmesi.
- `aes_request`: yalnızca saklanacak request'lerin subject, süre ve deadline bilgileri.
- `aes_schema_version`: tek satır, `id=1`, `version=1`.

İlk release için migration zinciri yoktur. Başka şema sürümü kabul edilmez.
Otomatik oluşturma kapalıysa mevcut sürüm tablosu doğrulanır.
Mevcut v1 kuruluma yeni `aes_request` tablosunu eklemek için
`E5_AES_DB_GENERATE_IF_NOT_EXIST=true` ile bir kez başlatın. Şema sürümü ilk
release kapsamındaki `1` olarak kalır; migration zinciri eklenmez.

## Retry protokolü ve temizleme

AES `RETRY_EVENT` gönderir. Broker yalnızca hata veren gruptan, subject'i
dinleyen tek aktif tüketiciye normal `EVENT` teslim eder. İlk tüketiciye bağlı
kalınmaz; aynı gruptaki başka bir instance kullanılabilir. Normal `EVENT`
yayınının tüm gruplara dağıtım davranışı değişmez.

Her retry yeni `messageId` taşır; `originalMessageId` ilk mesajı belirtir.
Tüketici consumption sonucunda **aldığı attempt messageId'yi** ve subject'i
göndermelidir. Group broker'daki gerçek bağlantıdan alınır.

`RETRY_ACCEPTED` broker'ın tüketici kuyruğuna aldığı anlamına gelir,
consumption başarısı değildir. `RETRY_NO_TARGET`/`RETRY_ERROR` veya bağlantı
kurulamaması retry limitini tüketmez; `try_after` sonrasında yeniden denenir.
Gönderimden sonra bağlantının belirsiz şekilde kopması durumunda ayrılmış
deneme korunur; consumption sonucu/timeout beklenir. AES tam gönderim sırasında
çökerse aynı koruma geçerlidir. Timeout kapalıysa belirsiz deneme saklanır.

Gecikmiş bir başarı aynı delivery'yi tamamlayabilir. Eski/tekrarlanan hatalar
yeni deneme planlamaz ve başarılı kaydı geri açmaz.

Tek scheduler goroutine'i her saniye, en fazla 100 zamanı gelmiş request'i ve
100 event delivery kaydını işler.
Süreler en erken işlem zamanıdır; DB/ağ yükü uygulamayı geciktirebilir.
Terminal kayıt silinirken delivery, attempt eşleştirmeleri ve ilgili grubun
logları birlikte temizlenir. Ortak `EVENT_RECEIVED` logu son takip edilen grup
temizlenince silinir. Diğer grupların logları korunur.

Exhaust/tap kalıcı, ACK'li bir kuyruk değildir; kopma/tampon taşması sırasında
olay kaybolabilir. AES aldığı ve başarıyla DB'ye yazdığı olayları takip eder.
Event retry duplicate işleme ihtimali taşır; tüketicide `originalMessageId` bazlı
idempotency önerilir.

Başlatma: ortam değişkenleri yüklendikten sonra `go run ./cmd/aes`.
SIGINT/SIGTERM scheduler'ı durdurur, tap bağlantısını kapatır ve DB'yi bırakır.

## JSON Schema ve örnek manifest

- [Aktif manifest](../../aes.rules.json): AES'in varsayılan olarak okuduğu dosya.
- [Örnek manifest](../../aes.rules.example.json): üç event ve iki request politikası gösterir;
  otomatik olarak etkinleştirilmez.
- [JSON Schema](../../aes.rules.schema.json): alan tiplerini, zorunlu alanları,
  enum seçeneklerini, varsayılanları ve sayısal sınırları tanımlar.

VS Code için [workspace ayarları](../../.vscode/settings.json) aktif ve örnek
manifesti yerel şemaya bağlar. Alan açıklamaları, otomatik tamamlama ve JSON
doğrulama çalışır; şema dosyasını indirmek için ağ bağlantısı gerekmez.
`default` değerleri bilgilendiricidir; eksik alanların varsayılanlarını AES uygular.

**Manifest içine `$schema` eklemeyin.** AES bilinmeyen kök alanları reddeder.
Şema ilişkisi editör ayarlarında tutulur. Başka bir dosya adı kullanıyorsanız
`json.schemas` içindeki `fileMatch` listesine onu ekleyin. Şema Go regex sözdizimini
doğrulamaz; regex derlemesi başlangıçta AES tarafından yapılır.

Tüm süreler tam sayı saniyedir, `0..9223372036` aralığındadır.
`try_count` negatif olmayan bir `int64` değeridir. Alan adları büyük/küçük harfe
duyarlıdır; örneğin `ON_CONSUMTATION_TIMEOUT` yerine
`on_consumption_timeout` kullanın.

### Örnek manifestin davranışı

1. `order.created`: yalnızca `orders` grubunun `orders-worker-` ile başlayan
   instance'larına yapılan ilk teslimatları takip eder. Hata sonrası 30 saniye
   bekler, en fazla 5 retry yapar. 120 saniyede sonuç gelmezse hata sayar.
   Başarıları 1 saat, terminal hataları 1 gün saklar.
2. `^payment\\.`: `payment.` ile başlayan subject'ler, tüm gruplar ve instance'lar
   için geçerlidir. Açık hatalarda 60 saniye arayla sınırsız retry yapar.
   300 saniyede sonuç gelmezse `keep` nedeniyle kaydı korur ve otomatik işlem
   yapmaz. Başarılı kayıtlar 1 saat sonra silinir.
3. `temporary.notification`: retry yapmaz. 600 saniyede sonuç gelmezse kayıt ve
   ilişkili logları siler; başarılı kayıtları 60 saniye saklar. Diğer alanlarda
   varsayılanlar uygulanır.
4. Request `audit/payment-details`: request/response loglarını 1 gün saklar.
5. Request `^audit/permanent/`: eşleşen request/response loglarını süresiz saklar.
   Diğer request'ler kaydedilmez.

Denemek için örnekten ihtiyacınız olan kuralları aktif manifestin `rules`
listesine kopyalayın veya `E5_AES_RULES_FILE=aes.rules.example.json` ayarlayıp
AES'i yeniden başlatın. İkinci seçenek örnekteki **tüm** kuralları etkinleştirir;
özellikle `remove` politikasının veri silebileceğini dikkate alın.
