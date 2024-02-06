# TODO

## Create directories
```
mkdir data certs
```


## TEST TEST TEST TEST

From DTK:

### gen root CA
openssl genrsa -out certs/ca.key 2048
openssl req -x509 -new -nodes -key certs/ca.key -sha256\
 -subj /CN=internalca -days 10000 -out certs/ca.crt

### gen server cert and key
openssl genrsa -out certs/localhost.key 2048

### CSR
cat <<EOF > certs/localhost.conf
[ req ]
default_bits = 2048
prompt = no
default_md = sha256
req_extensions = req_ext
distinguished_name = dn

[ dn ]
CN = localhost

[ req_ext ]
subjectAltName = @alt_names

[ alt_names ]
DNS.1 = localhost
IP.1 = 127.0.0.1

[ v3_ext ]
authorityKeyIdentifier=keyid,issuer:always
basicConstraints=CA:FALSE
keyUsage=keyEncipherment,dataEncipherment,digitalSignature,nonRepudiation
extendedKeyUsage=serverAuth,clientAuth
subjectAltName=@alt_names
EOF


openssl req -new -key certs/localhost.key\
 -out certs/localhost.csr -config certs/localhost.conf

openssl x509 -req -in certs/localhost.csr -CA certs/ca.crt -CAkey certs/ca.key\
 -CAcreateserial -out certs/localhost.crt -days 10000 -extensions v3_ext -extfile certs/localhost.conf










## Create certs for Nginx to terminate TLS

```
openssl req\
  -x509\
  -nodes\
  -days 365\
  -newkey rsa:2048\
  -keyout ./certs/localhost.key\
  -out    ./certs/localhost.crt\
  -config ./certs/localhost.conf
```

## Run nginx

```
docker run\
  --rm\
  --name nginx\
  --network host\
  -v $PWD/conf:/etc/nginx\
  -v $PWD/certs:/certs\
  -p 8443:8443\
  nginx:latest
```

## Run docker registry

(Same as `test-tegistry-servers/no-auth-no-tls`):
```
docker run\
  --rm\
  --name registry\
  -p 5000:5000\
  registry:2.8.3
```

Use `crane` to push an image tarball into the registry

crane pull hello-world:latest hello-world.latest.tar

crane push hello-world.latest.tar localhost:5000/hello-world:latest

curl -k  https://localhost:8443/v2/hello-world/manifests/latest
