- [x] JSON => MessagePack
- [x] Mesaj listesi => Bekleyen Listesi / Gönderilen listesi
- [x] İstek / yanıtlama
- [x] Eşzamanlı yazmayı önleme ve semafor yapısı
- [ ] "Listen" keyword zaten ekliyse ekleme!
- [ ] Daha iyi Instance Yönetimi
        Map
        { instanceName: {
            instances: [Instance listesi]  
            listeningSubjects: [string listesi] // string listesi map true olabilir default olarak false verilir. 
            currentIndex: 0 => i % instances.length
        } }


