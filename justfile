default: build

build:
    go build -o jui .

install:
    go install .

run *args:
    go run . {{args}}

fmt:
    gofmt -w .

vet:
    go vet ./...

clean:
    rm -f jui
