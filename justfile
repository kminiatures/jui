default: build

build:
    go build -ldflags="-s -w" -o jui .

install:
    go install -ldflags="-s -w" .

run *args:
    go run . {{args}}

fmt:
    gofmt -w .

vet:
    go vet ./...

clean:
    rm -f jui
