
set -e

go mod tidy
go fmt
go build -buildvcs=false

ls -lh
