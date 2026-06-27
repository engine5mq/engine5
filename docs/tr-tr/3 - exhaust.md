# Engine5 Egzoz Çıkışı (Exhaust / Log)

Bu doküman, Engine5'in **egzoz çıkışı** özelliğini açıklar. Egzoz çıkışı,
sunucu içindeki olayları (event geldi, şu client'a iletildi, şuna iletilemedi,
parse hatası, bağlantı açıldı/kapandı vb.) hem konsola hem de dışarıdaki bir
uygulamaya yapısal biçimde akıtan merkezi log/observability katmanıdır.

İki çıkış vardır:

1. **Konsol (slog)** — ortam değişkenine göre açılıp kapanır. **Production'da
   varsayılan olarak kapalıdır**, yani konsola log basmaz.
2. **Tap (ayrı port)** — olayları ayrı bir TCP portundan **NDJSON** olarak
   yayınlar. Harici uygulamalar (örn. `e5-tap`) bu portu dinleyerek tüm akışı
   gerçek zamanlı alabilir.

---

## 1) Konsol çıkışı ve ortam ayarları

Konsol çıkışı `slog` tabanlıdır ve şu ortam değişkenleriyle yönetilir:

| Değişken | Varsayılan | Açıklama |
|---|---|---|
| `E5_ENV` | `development` | `production` → konsol **kapalı** |
| `E5_LOG_LEVEL` | dev=`DEBUG`, prod=`OFF` | `DEBUG` / `INFO` / `WARN` / `ERROR` / `OFF` |
| `E5_LOG_FORMAT` | dev=`text`, prod=`json` | Konsol formatı |
| `E5_EXHAUST_INCLUDE_CONTENT` | `false` | Payload `content` alanı maskelenir (`[hidden]`) |

> Not: Production'da `E5_LOG_LEVEL` elle bir seviyeye ayarlanırsa konsol tekrar
> açılır.

### Gizlilik

Payload `content` alanı hassas veri içerebilir. Bu yüzden egzoz olaylarında
`content` **varsayılan olarak maskelenir**. Görmek için:

```env
E5_EXHAUST_INCLUDE_CONTENT=true
```

---

## 2) Tap çıkışını açmak

Tap, ayrı bir TCP portunda dinler ve olayları NDJSON (her satır bir JSON olay)
olarak yayınlar.

| Değişken | Varsayılan | Açıklama |
|---|---|---|
| `E5_EXHAUST_ENABLE` | `false` | Tap portunu açar |
| `E5_EXHAUST_PORT` | `3536` | Tap portu |
| `E5_EXHAUST_KEY` | (boş) | Ortak anahtar; boşsa anahtar doğrulaması yapılmaz |
| `E5_EXHAUST_TLS` | `ENABLE_TLS` ile aynı | Tap portunun TLS kullanıp kullanmayacağı |

Örnek sunucu başlatma:

```bash
E5_EXHAUST_ENABLE=true \
E5_EXHAUST_KEY=super-secret-key \
go run ./cmd/engine5
```

### Güvenlik notları

- `E5_EXHAUST_KEY` ayarlıysa, dinleyici bağlandığında **ilk satır** olarak bu
  anahtarı göndermek zorundadır. Anahtar sabit-zamanlı karşılaştırma ile
  doğrulanır; yanlışsa bağlantı kapatılır.
- Tap portu, ana sunucu ile aynı TLS sertifikası üzerinden şifrelenebilir.
- Yavaş tüketici koruması: her dinleyicinin kendi tamponu vardır. Tampon
  dolarsa o dinleyici için olaylar düşürülür; bu durum mesaj broker'ın
  performansını etkilemez.

---

## 3) Olay formatı (NDJSON)

Her olay tek satır JSON'dur. Alanlar:

```json
{
	"time": "2026-06-27T16:17:44.289Z",
	"level": "INFO",
	"kind": "EVENT_DELIVERED",
	"instance": "users-service-1",
	"group": "users-service",
	"subject": "user.created",
	"messageId": "abc-123",
	"remote": "10.0.0.5:5567",
	"content": "[hidden]",
	"err": "",
	"msg": "Event delivered to client"
}
```

### Olay türleri (`kind`)

| Kind | Anlamı |
|---|---|
| `SERVER_START` / `SERVER_ERROR` | Sunucu yaşam döngüsü |
| `CLIENT_CONNECTING` | Yeni bağlantı geldi |
| `CLIENT_CONNECTED` | Client bağlandı |
| `CLIENT_CLOSING` / `CLIENT_CLOSED` | Bağlantı kapanıyor / kapandı |
| `CLIENT_RENAMED` | Çakışan instance adı yeniden adlandırıldı |
| `AUTH_OK` / `AUTH_REJECTED` | Kimlik doğrulama sonucu |
| `CLIENT_LISTEN` | Client bir subject dinlemeye başladı |
| `EVENT_RECEIVED` | Client event gönderdi |
| `EVENT_DELIVERED` | Event bir client'a iletildi |
| `EVENT_NO_LISTENER` | Event'i dinleyen client yok |
| `REQUEST_RECEIVED` | Client request gönderdi |
| `REQUEST_ROUTED` | Request bir client'a yönlendirildi |
| `REQUEST_NO_TARGET` | Kritere uyan client bulunamadı |
| `RESPONSE_RECEIVED` | Client request'e cevap verdi |
| `RESPONSE_DELIVERED` | Cevap, isteyen client'a iletildi |
| `PROTOCOL_ERROR` / `PARSE_ERROR` / `INTERNAL_ERROR` | Hatalar |

---

## 4) `e5-tap` ile dinlemek

`e5-tap`, egzoz portunu dinleyen bağımsız bir komut satırı aracıdır.

### Çalıştırma

```bash
# TLS + anahtar ile (production)
E5_EXHAUST_KEY=super-secret-key go run ./cmd/e5-tap -host localhost -port 3536

# Düz TCP (development)
go run ./cmd/e5-tap -tls=false

# Self-signed sertifika ile (TLS doğrulamasını atla)
go run ./cmd/e5-tap -insecure

# Ham JSON çıktısı (örn. jq ile işlemek için)
go run ./cmd/e5-tap -raw | jq
```

### Bayraklar (flags)

| Bayrak | Ortam değişkeni | Varsayılan | Açıklama |
|---|---|---|---|
| `-host` | `E5_EXHAUST_HOST` | `localhost` | Tap sunucusu adresi |
| `-port` | `E5_EXHAUST_PORT` | `3536` | Tap portu |
| `-key` | `E5_EXHAUST_KEY` | (boş) | Ortak anahtar |
| `-tls` | `E5_EXHAUST_TLS` | `true` | TLS ile bağlan |
| `-insecure` | — | `false` | TLS sertifika doğrulamasını atla (dev) |
| `-ca` | `E5_EXHAUST_CA_FILE` | (boş) | Sunucuyu doğrulamak için CA dosyası |
| `-raw` | — | `false` | Biçimlendirilmiş yerine ham NDJSON bas |
| `-reconnect` | — | `true` | Bağlantı koparsa otomatik yeniden bağlan |

### Örnek çıktı

```
16:17:44.288  INFO   CLIENT_CONNECTING   remote=[::1]:45610  Incoming connection
16:17:44.289  WARN   CLIENT_RENAMED      instance=6240463f-...  Duplicate instance name, renamed
16:17:44.289  INFO   EVENT_DELIVERED     instance=users-service-1 subject=user.created  Event delivered to client
16:17:44.289  INFO   CLIENT_CLOSING      instance=6240463f-...  Connection closed by client
```

---

## 5) Tipik kullanım senaryoları

- **Production gözlemlenebilirlik**: Konsol kapalı tutulur (`E5_ENV=production`),
  tap açılır (`E5_EXHAUST_ENABLE=true`). Harici bir izleme uygulaması veya
  `e5-tap` portu dinleyerek tüm akışı toplar.
- **Hata ayıklama**: `E5_EXHAUST_INCLUDE_CONTENT=true` ile payload içerikleri de
  akışa dahil edilir.
- **Entegrasyon**: NDJSON formatı sayesinde akış, `jq` veya herhangi bir dildeki
  bir tüketici tarafından kolayca işlenebilir.
