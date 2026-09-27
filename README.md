# srvmon — Sunucu ve Veritabanı İzleme

Windows Server üzerinde çalışan, sistem sağlığını ve Oracle veritabanlarını
izleyen (Oracle, PostgreSQL, MySQL/MariaDB), canlı bir web panosu sunan ve eşik aşımında e-posta / SMS alarmı
gönderen tek dosyalık bir uygulama.

- **Tek exe, kurulum yok.** CGO gerektirmez, harici bağımlılık gerekmez.
- **Veritabanı istemcisi gerekmez.** Saf Go sürücüleri: `go-ora` (Oracle), `pgx` (PostgreSQL), `go-sql-driver/mysql` (MySQL/MariaDB).
- **Canlı pano.** Tarayıcıda açılır, kendini periyodik yeniler, grafik çizer.
- **Alarm.** Eşik aşımında SMTP e-posta ve HTTP tabanlı SMS (her sağlayıcı).

## Ne izler?

**Sistem:** CPU %, bellek, swap, her disk bölümünün doluluğu, ağ arayüzü
indirme/yükleme hızı, çalışma süresi, süreç sayısı.

**Veritabanları (Oracle / PostgreSQL / MySQL / MariaDB):** erişilebilirlik ve gecikme, örnek durumu ve
çalışma süresi, oturum sayısı / üst sınır oranı, bloklayan oturumlar, buffer
hit oranı, tablespace doluluk oranları.

**Derin teşhis (Windows):**
- **Sorunlu servisler:** Otomatik başlatmalı olduğu halde çalışmayan
  hizmetleri (ör. durmuş SQL Server Agent, IIS) tespit eder.
- **Olay Günlüğü:** System ve Application günlüklerindeki son kritik ve hata
  kayıtlarını okuyup panoda gösterir.
- **Durum değerlendirmesi:** Tüm bunları "C: diski %92 dolu", "SQL Server Agent
  servisi çalışmıyor", "olay günlüğünde 3 kritik kayıt" gibi sıralı, okunur
  bulgulara ve genel bir sağlık verdiğine (Normal / Uyarı / Kritik) dönüştürür.

**İzlenen uygulama / servisler (kendi uygulamanız dahil):** `watch` bölümünde
adını verdiğiniz her öğe tek tek kontrol edilir ve durdurulursa hem panoda hem
alarmda (`watch.down`) gösterilir. Üç yolla izlenebilir:
- **Servis adı** — `"services": ["MSSQLSERVER","SiparişUygulaması"]`
- **Süreç adı** — `"processes": ["MyApp.exe"]`
- **Sağlık ucu** — kendi uygulamanız için en anlamlısı: gerçekten yanıt
  veriyor mu? `"endpoints":[{"name":"Sipariş API","url":"http://localhost:8080/health"}]`
  (HTTP durum kodu) veya `{"name":"...","tcp":"127.0.0.1:5000"}` (bağlantı testi).

**Durum raporu e-postası:** Alarmlardan bağımsız, periyodik özet e-posta.
Sağlık verdiği, bulgular, öngörüler, izlenen öğeler, diskler ve Oracle özetini
HTML olarak gönderir. `digest` bölümünden açılır; SMTP ayarları `email`
bölümünden alınır. Zamanlama:
- `"every_hours": 8` — 8 saatte bir, veya
- `"daily_at": "08:30"` — her gün belirli saatte (ayarlıysa `every_hours`
  yerine geçer).

E-postanın nasıl görüneceğini kurmadan görmek için:

```
srvmon.exe -config config.json -demo -digest-file rapor.html
srvmon.exe -config config.json -digest-now      # gerçek SMTP ile bir kez gönder
```

**Erken uyarı (öngörü):** Asıl amaç sorunu olmadan haber vermektir. Uygulama,
disk / bellek / Oracle oturumu / tablespace gibi "dolan" metriklerin son
geçmişteki artış hızını hesaplayıp **ne kadar süre sonra dolacağını** tahmin
eder: *"C: diski bu hızla ~5sa 20dk içinde dolabilir"*. Bu öngörü, eşik daha
aşılmadan panoda gösterilir ve istenirse e-posta/SMS ile bildirilir
(`forecast.hours_to_full` kuralı). Böylece disk %92'ye ulaşmadan önce, artış
eğilimi belirdiği anda haberdar olursunuz.

Servis ve olay taraması, sistem metriklerinden bağımsız ve daha seyrek çalışır
(`deep_seconds`, varsayılan 60 sn).

### Önizleme

Gerçek sunucuya kurmadan panonun tam görünümünü sentetik veriyle görmek için:

```
srvmon.exe -config config.json -demo
```

## Ayarlar (tarayıcıdan)

Tüm yapılandırma tarayıcıdan yapılabilir: panonun sağ üstündeki **⚙ Ayarlar**
bağlantısı (veya `http://localhost:8085/ayarlar`). Buradan veritabanı
bağlantıları, SMTP e-posta ayarları, SMS API'si, durum raporu, izlenen öğeler
ve alarm kuralları düzenlenip **Kaydet ve uygula** ile anında geçerli olur —
uygulama yeniden başlatılmadan yeni ayarları yükler. Değişiklikler
`config.json`'a yazılır. (Yalnızca `web.listen` ve `web.token` değişiklikleri
yeniden başlatma gerektirir.)

Ayar sayfasını dışarı açacaksanız `web.token` belirleyin; ayarlanmışsa API
`?token=...` ister.

## Kurulum

1. `config.example.json` dosyasını `config.json` olarak kopyalayın ve
   düzenleyin (sunucu adı, Oracle bağlantıları, eşikler, e-posta/SMS).
2. `srvmon.exe` ile `config.json`'u aynı klasöre koyun.
3. Çalıştırın:

   ```
   srvmon.exe -config config.json
   ```

4. Tarayıcıda açın: `http://localhost:8085`

Windows hizmeti olarak çalıştırmak için (yönetici PowerShell):

```powershell
sc.exe create srvmon binPath= "C:\srvmon\srvmon.exe -config C:\srvmon\config.json" start= auto
sc.exe start srvmon
```

## Veritabanı izleme kullanıcısı

İzleme için salt-okunur bir kullanıcı yeterlidir:

```sql
CREATE USER izleyici IDENTIFIED BY parola;
GRANT CREATE SESSION TO izleyici;
GRANT SELECT_CATALOG_ROLE TO izleyici;   -- v$ ve dba_ görünümleri için
```

DSN biçimleri:
- Oracle: `oracle://kullanici:parola@sunucu:1521/servis`
- PostgreSQL: `postgres://kullanici:parola@sunucu:5432/veritabani?sslmode=disable`
- MySQL/MariaDB: `kullanici:parola@tcp(sunucu:3306)/veritabani`

`config.json` içinde her veritabanı `type` alanıyla belirtilir (oracle | postgres | mysql).

## Kurallar (alarm eşikleri)

Her kural bir metrik anahtarını izler:

| Anahtar | Açıklama |
|---|---|
| `cpu.percent`, `mem.percent`, `swap.percent` | Sistem kullanım oranları |
| `disk.*.percent` | Her disk bölümü (`*` tüm bölümleri kapsar) |
| `net.<ad>.rx_bps`, `net.<ad>.tx_bps` | Ağ hızı (bayt/sn) |
| `db.<ad>.up` | 1 = erişilebilir, 0 = erişilemiyor |
| `db.<ad>.session_percent` | Bağlantı / üst sınır oranı |
| `db.<ad>.blocking` | Bloklayan/bekleyen oturum sayısı |
| `db.<ad>.buffer_hit_ratio` | Buffer cache hit oranı |
| `db.<ad>.space.*.percent` | Depolama alanı doluluğu (Oracle tablespace) |
| `services.problem` | Otomatik ama çalışmayan servis sayısı |
| `events.critical` | Son penceredeki kritik olay sayısı |
| `events.error` | Son penceredeki hata olayı sayısı |
| `forecast.hours_to_full` | En yakın kaynağın dolmasına tahmini saat (erken uyarı — `compare: lt` ile kullanın) |
| `watch.down` | İzlenen (servis/süreç/uç nokta) öğelerden çalışmayanların sayısı |

Kural alanları:

- `compare`: `gt` (üstündeyse alarm) veya `lt` (altındaysa alarm — boş disk,
  `up` gibi).
- `warn` / `crit`: uyarı ve kritik eşikleri.
- `for`: eşik bu süre boyunca aşılırsa alarm üretilir (anlık dalgalanmaları
  eler). `0s` = anında.
- `cooldown`: aynı seviyedeki alarmın tekrar bildirilme aralığı. Uyarıdan
  kritiğe yükselme her zaman anında bildirilir; normale dönüş de bildirilir.
- `channels`: `["email"]`, `["sms"]` veya `["email","sms"]`.

## SMS sağlayıcısı

`sms.url`, `sms.headers` ve `sms.body` alanlarında `{message}`, `{to}` ve
`{severity}` yer tutucuları kullanılabilir. Böylece Twilio, Netgsm,
İletimerkezi gibi HTTP API'si olan her sağlayıcı koda dokunmadan bağlanır.

## Yetki gereksinimi

Servisleri ve Olay Günlüğünü tam görebilmek için uygulamanın **yönetici**
(veya LocalSystem — Windows hizmeti olarak çalışırken varsayılan) yetkisiyle
çalışması gerekir. Yetki yoksa sistem ve Oracle metrikleri yine çalışır;
yalnızca servis/olay bölümleri boş kalır.

## Güvenlik notları

- Panoyu yalnızca güvenli iç ağda açın; dışarı açacaksanız `web.token`
  belirleyin ve önüne TLS sonlandıran bir ters proxy koyun.
- İzleme kullanıcısı salt-okunur olmalıdır; yazma yetkisi vermeyin.
- Bu araç yalnızca okur ve rapor eder; sunucuda değişiklik yapmaz — servis
  başlatmaz/durdurmaz, olay silmez.

## Geliştirme

```
go test ./...              # birim testleri
go test -race ./...        # yarış koşulu kontrolü
go vet ./...
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o srvmon.exe ./cmd/srvmon
```

## Lisans

Uygulama kodu: MIT. Bağımlılıklar: `go-ora` (MIT), `pgx` (MIT), `gopsutil` (BSD-3-Clause), `go-sql-driver/mysql` (MPL-2.0 — değiştirilmeden bağımlılık olarak kullanım kodunuzun lisansını etkilemez).
