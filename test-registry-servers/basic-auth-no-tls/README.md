# Configure basic auth

These steps assume your current working directory is the project root.

## Prepare

Install `htpasswd` utility:
```
sudo apt install apache2-utils
```

## Create credentials file
```
htpasswd -bnB ericace ericace >| test-registry-servers/basic-auth-no-tls/auth/htpasswd
```

## Verify
```
cat test-registry-servers/basic-auth-no-tls/auth/htpasswd
```

## Result
```
ericace:$2y$05$4B7xWnrLxZiCJkG/kBIYkufcT9yPg3C3leUQT9MMxqoOP6geYhmd2
```
