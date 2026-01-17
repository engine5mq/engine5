# Engine5Go Güvenlik Rehberi

Bu dokümanda Engine5 TCP server'ınızı güvenli bir şekilde production ortamında çalıştırmak için gerekli adımları bulacaksınız.

## 🔒 Güvenlik Özellikleri

### 1. TLS/SSL Şifrelemesi
- **Transport Layer Security (TLS 1.2+)** ile tüm iletişim şifrelenir
- **Perfect Forward Secrecy** desteği
- Güçlü cipher suite'ler (AES-256-GCM, ChaCha20-Poly1305)
- İsteğe bağlı **mutual TLS authentication** (client certificate)

### 2. Authentication & Authorization
- **Token-based authentication** (HMAC-SHA256 imzalı)
- **Role-based access control** (RBAC)
- **Subject-level permissions** (publish/subscribe/request)
- **Rate limiting** (client başına)

### 3. Connection Security
- **Connection limits** (maksimum eşzamanlı bağlantı)
- **Connection timeout** (idle connection'lar için)
- **IP whitelisting** desteği
- **Non-root user** execution

### 4. Monitoring & Logging
- **Audit logging** (tüm güvenlik olayları)
- **Connection logging** (kim, ne zaman, nereden)
- **Rate limit violations** tracking
- **Health check endpoint**

## 🚀 Kurulum

### 1. SSL Sertifikalarını Oluşturun
```bash
./generate_certs.sh
```

### 2. Environment Dosyasını Hazırlayın
```bash
cp .env.example .env
# .env dosyasını düzenleyin
```

### 3. Güvenlik Ayarlarını Yapılandırın
```bash
# TLS'i etkinleştir
export ENABLE_TLS=true
export TLS_CERT_FILE=./certs/server.crt
export TLS_KEY_FILE=./certs/server.key

# Authentication'ı etkinleştir  
export REQUIRE_AUTH=true
export AUTH_SECRET=$(openssl rand -base64 32)

# Connection limits
export MAX_CONNECTIONS=1000
export CONNECTION_TIMEOUT=30
```

### 4. Uygulamayı Başlatın
```bash
go run .
```

## 🐳 Docker ile Güvenli Çalıştırma

```bash
# Güvenli docker-compose ile çalıştır
docker-compose -f docker-compose.secure.yml up -d
```

## 🔑 Client Authentication

### Token Almak
Client'lar önce authentication yapmak zorunda:

```json
{
    "command": "AUTH",
    "content": "client-id-here"
}
```

Başarılı authentication sonrası token alınır:
```json
{
    "command": "AUTH_SUCCESS", 
    "content": "eyJ0eXAiOiJKV1Qi..."
}
```

### Token ile Bağlantı
Her sonraki işlem için token gönderilmeli:
```json
{
    "command": "CONNECT",
    "instance_id": "my-client-123",
    "token": "eyJ0eXAiOiJKV1Qi..."
}
```

## 📊 Permissions Sistemi

### Client Permission Tanımlama
```json
{
    "admin": {
        "can_publish": true,
        "can_subscribe": true, 
        "can_request": true,
        "allowed_subjects": ["*"],
        "rate_limit": 120
    },
    "guest": {
        "can_publish": false,
        "can_subscribe": true,
        "can_request": false, 
        "allowed_subjects": ["public.*", "news.*"],
        "rate_limit": 30
    }
}
```

### Subject Pattern Matching
- `*` - Tüm subject'ler
- `user.*` - user. ile başlayan subject'ler  
- `system.admin` - Tam eşleşme

## 🚨 Production Güvenlik Checklist

### ✅ Temel Güvenlik
- [ ] TLS etkinleştirildi (`ENABLE_TLS=true`)
- [ ] Güçlü AUTH_SECRET kullanıldı (32+ karakter random)
- [ ] REQUIRE_AUTH etkinleştirildi
- [ ] Connection limits ayarlandı
- [ ] Non-root user kullanılıyor

### ✅ Sertifikalar  
- [ ] Production SSL sertifikaları kullanılıyor (self-signed değil)
- [ ] Sertifikalar güvenli yerde saklanıyor
- [ ] Sertifika geçerlilik tarihleri takip ediliyor
- [ ] Private key'ler 600 permissions ile korunuyor

### ✅ Network Security
- [ ] Firewall kuralları ayarlandı
- [ ] IP whitelisting yapılandırıldı (gerekirse)
- [ ] Reverse proxy kullanılıyor (nginx/apache)
- [ ] DDoS protection aktif

### ✅ Monitoring
- [ ] Log monitoring sistemi kuruldu
- [ ] Alert kuralları tanımlandı
- [ ] Health check endpoint'i izleniyor  
- [ ] Resource usage takip ediliyor

### ✅ Backup & Recovery
- [ ] Configuration backup'ları alınıyor
- [ ] SSL sertifikaları backup'lanıyor
- [ ] Disaster recovery planı hazır
- [ ] Security incident response planı var

## 🔧 Troubleshooting

### TLS Bağlantı Sorunları
```bash
# Sertifika geçerliliğini test et
openssl x509 -in certs/server.crt -text -noout

# TLS bağlantısını test et  
openssl s_client -connect localhost:3535
```

### Authentication Sorunları
```bash
# Token decode (debug için)
echo "TOKEN_BURADA" | base64 -d
```

### Performance Sorunları
```bash
# Connection sayısını kontrol et
netstat -an | grep :3535 | wc -l

# Resource usage
docker stats engine5go-server
```

## 📞 Destek

Güvenlik sorunları için:
- **Security**: security@yourcompany.com
- **Documentation**: Bu README'yi güncel tutun
- **Updates**: Düzenli security update'leri takip edin

---
**⚠️ ÖNEMLİ**: Bu sistem production'da para kazanmanızı sağlıyorsa, professional security audit yaptırmayı düşünün!