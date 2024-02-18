# Configure Nginx TLS termination

## Create directories

```
mkdir\
  test-registry-servers/nginx/conf\
  test-registry-servers/nginx/certs
```

## Create certs

### gen root CA

Will be used to sign the cert that will be used by both the client and the server (nginx):
```
openssl genrsa -out test-registry-servers/nginx/certs/ca.key 2048
openssl req -x509 -new -nodes -key test-registry-servers/nginx/certs/ca.key -sha256\
 -subj /CN=internalca -days 10000 -out test-registry-servers/nginx/certs/ca.crt
```

### Gen server cert and key

```
openssl genrsa -out test-registry-servers/nginx/certs/localhost.key 2048
```

### Create a certificate signing request

```
cat <<EOF > test-registry-servers/nginx/certs/localhost.conf
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
```

### Generate a cert and key

This cert and key will be used by both the client and the server to identify themselves. The key point is that since the cert is signed by the CA - both the server and the client can use the CA to verify each other.

```
openssl req -new\
  -key test-registry-servers/nginx/certs/localhost.key\
  -out test-registry-servers/nginx/certs/localhost.csr\
  -config test-registry-servers/nginx/certs/localhost.conf

openssl x509 -req\
  -in test-registry-servers/nginx/certs/localhost.csr\
  -CA test-registry-servers/nginx/certs/ca.crt\
  -CAkey test-registry-servers/nginx/certs/ca.key\
  -CAcreateserial\
  -out test-registry-servers/nginx/certs/localhost.crt\
  -days 10000\
  -extensions v3_ext\
  -extfile test-registry-servers/nginx/certs/localhost.conf
```
