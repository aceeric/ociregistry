

https://learning-ocean.com/tutorials/docker/docker-docker-registry-basic-authentication
---------------------------------------------------------------------------------------
sudo apt install apache2-utils

htpasswd -bnB ericace ericace > auth/htpasswd

cat auth/htpasswd 
ericace:$2y$05$4B7xWnrLxZiCJkG/kBIYkufcT9yPg3C3leUQT9MMxqoOP6geYhmd2

docker run -d -p 5000:5000 --name registry\
 -v $(pwd)/auth:/auth\
 -e REGISTRY_AUTH=htpasswd\
 -e REGISTRY_AUTH_HTPASSWD_REALM="Registry Realm"\
 -e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd\
 registry:2.8.3


LATER:
 -v $(pwd)/certs:/certs
 -e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt\
 -e REGISTRY_HTTP_TLS_KEY=/certs/domain.key\
