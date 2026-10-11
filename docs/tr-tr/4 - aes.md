# AES: kalıcı retry ve kayıt temizleme

AES tap olaylarını veritabanına kaydeder. Kuralla eşleşen event teslimatları,
mesaj + subject + tüketici grubu bazında ayrıca takip edilir. Bir gruptaki hata
diğer grupların başarılı teslimatlarını etkilemez.

## Yapılandırma

`aes.env` yanında `aes.rules.json` bulunur. Dosya yolu çalışma dizinine göre
çözülür; farklı bir konum için `E5_AES_RULES_FILE` kullanın. Env dosyasını
çalıştırıcı/IDE ortam değişkenlerine yüklemelidir; AES kendisi env dosyası okumaz.

Depodaki dosya boş bir kural listesiyle gelir: mevcut loglama devam eder, otomatik
retry ve temizleme devre dışıdır. Örnek:

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

Kuralla eşleşmeyen teslimatlar yalnızca loglanır, otomatik temizlenmez.
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
- `aes_schema_version`: tek satır, `id=1`, `version=1`.

İlk release için migration zinciri yoktur. Başka şema sürümü kabul edilmez.
Otomatik oluşturma kapalıysa mevcut sürüm tablosu doğrulanır.

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

Tek scheduler goroutine'i her saniye, en fazla 100 zamanı gelmiş kaydı işler.
Süreler en erken işlem zamanıdır; DB/ağ yükü uygulamayı geciktirebilir.
Terminal kayıt silinirken delivery, attempt eşleştirmeleri ve ilgili grubun
logları birlikte temizlenir. Ortak `EVENT_RECEIVED` logu son takip edilen grup
temizlenince silinir. Diğer grupların logları korunur.

Exhaust/tap kalıcı, ACK'li bir kuyruk değildir; kopma/tampon taşması sırasında
olay kaybolabilir. AES aldığı ve başarıyla DB'ye yazdığı olayları takip eder.
Retry duplicate işleme ihtimali taşır; tüketicide `originalMessageId` bazlı
idempotency önerilir.

Başlatma: ortam değişkenleri yüklendikten sonra `go run ./cmd/aes`.
SIGINT/SIGTERM scheduler'ı durdurur, tap bağlantısını kapatır ve DB'yi bırakır.
