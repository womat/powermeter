set GOARCH=arm
set GOOS=linux
go build -o ..\bin\powermeter ..\cmd\powermeter.go

set GOARCH=386
set GOOS=windows
go build -o ..\bin\powermeter.exe ..\cmd\powermeter.go