#!/usr/bin/bash

apt install screen -y
# Preparing Golang ----------------------

screen -S work

apt install golang-go -y

mkdir -p /root/go/src/mnemonic_bruteforce
cp ./main.go /root/go/src/mnemonic_bruteforce/main.go
cd /root/go/src/mnemonic_bruteforce

go mod init

go get github.com/miguelmota/go-ethereum-hdwallet
go get github.com/tyler-smith/go-bip39
go get github.com/valyala/fasthttp
go get github.com/valyala/fasthttp/fasthttpproxy
go get github.com/aherve/gopool


# Preparing Tor -------------------------

amount=40

base_port=9060

apt install tor -y

cd /etc/tor/
rm ./torrc.*
for ((c=1 ; c <= $amount ; c++))
do 
   cp ./torrc ./torrc.$c
done

for ((c=1 ; c <= $amount ; c++))
do
   sport=$(($base_port+($c-1)*2))
   cport=$(($base_port+($c-1)*2+1))
   echo -e "SocksPort $sport\nControlPort $cport\nDataDirectory /var/lib/tor$c" >> ./torrc.$c
done

pkill tor
for ((c=1 ; c <= $amount ; c++))
do
   screen -S tor$c -dm tor -f /etc/tor/torrc.$c
done


# Running code -------------------------
cd /root/go/src/mnemonic_bruteforce
go run main.go
