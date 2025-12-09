go-goph-keeper

Если сервер записал в базу новый секрет от клиента, а клиент не принял ответ, то получим дубли.

go generate ./...
go run -ldflags "-X github.com/serg2014/go-goph-keeper/internal/client/tui.Version=v1.0.1 \
-X 'github.com/serg2014/go-goph-keeper/internal/client/tui.BuildTime=$(date +'%Y/%m/%d %H:%M:%S')'" \
./cmd/client/
