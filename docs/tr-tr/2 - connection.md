# Engine5 ile bağlantı sağlamak

Bu doküman, Engine5 sunucusuna minimum adımla bağlanmak için kısa bir rehberdir.

## 1) Sunucu ayarları

- Varsayılan port: `3535` (`E5_PORT` ile değiştirilebilir)
- TLS varsayılan: **açık** (`ENABLE_TLS=true`)
- Auth varsayılan: **zorunlu** (`REQUIRE_AUTH=true`)

Örnek `.env`:

```env
E5_PORT=3535
ENABLE_TLS=true
REQUIRE_AUTH=true
AUTH_SECRET=super-secret-key
```

## 2) Bağlantı protokolü

Engine5, ham JSON değil **MessagePack** kullanır.
Her mesaj şu formatta gönderilmelidir:

1. İlk 4 byte: mesaj uzunluğu (`big-endian`)
2. Devamı: MessagePack ile encode edilmiş `Payload`

`Payload` içindeki temel alanlar:

- `command`
- `instanceId`
- `instance_group`
- `authKey`

## 3) İlk bağlantı (CONNECT)

İstemci ilk mesaj olarak `CONNECT` göndermelidir:

```json
{
	"command": "CONNECT",
	"instanceId": "users-service-1",
	"instance_group": "users-service",
	"authKey": "super-secret-key"
}
```

### Sunucu cevapları

Başarılı bağlantı:

```json
{
	"command": "CONNECT_SUCCESS",
	"instanceId": "users-service-1",
	"instance_group": "users-service"
}
```

Hatalı bağlantı:

```json
{
	"command": "CONNECT_ERROR",
	"content": "Authentication failed: invalid auth key"
}
```

## 4) Önemli notlar

- `REQUIRE_AUTH=true` ise `authKey` göndermek zorunludur.
- `instanceId` boş gelirse sunucu otomatik UUID üretir.
- `instance_group` boş gelirse `instanceId` değeri kullanılır.
- Bağlandıktan sonra tipik komutlar: `LISTEN`, `EVENT`, `REQUEST`, `RESPONSE`, `CLOSE`. Çok yakında bu komutlar açıklanacak ancak kaynak kodları da inceleyebilirsiniz