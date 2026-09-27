# Windows x64 tek dosya derleme
$env:CGO_ENABLED="0"; $env:GOOS="windows"; $env:GOARCH="amd64"
go build -ldflags="-s -w" -o srvmon.exe ./cmd/srvmon
Write-Host "srvmon.exe olusturuldu."
