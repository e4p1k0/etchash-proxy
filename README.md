etchash-proxy

```sh
git clone https://github.com/e4p1k0/etchash-proxy.git
cd etchash-proxy/
go get github.com/etclabscore/go-etchash
go get github.com/ethereum/go-ethereum/common
go get github.com/goji/httpauth
go get github.com/gorilla/mux
go get github.com/yvasiyarov/gorelic
go build -o etchash-proxy main.go
```