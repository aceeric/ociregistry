
also take a look maybe at:
- https://medium.com/@ifeanyiigili/how-to-setup-a-private-docker-registry-with-a-self-sign-certificate-43a7407a1613
- https://devopsian.net/posts/docker-registry/docker-registry/
- https://medium.com/@ManagedKube/docker-registry-2-setup-with-tls-basic-auth-and-persistent-data-8b98a2a73eec
- https://docs.nginx.com/nginx/admin-guide/security-controls/terminating-ssl-tcp/
- https://www.ssltrust.com/help/setup-guides/client-certificate-authentication
- https://dev.to/darshitpp/how-to-implement-two-way-ssl-with-nginx-2g39


MAIN >>>>

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
