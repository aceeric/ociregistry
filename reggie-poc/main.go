package main

import (
	"fmt"

	"github.com/bloodorangeio/reggie"
)

//THIS WORKS:
//
//TOKEN=$(curl -s "https://auth.docker.io/token?scope=repository:library/hello-world:pull&service=registry.docker.io" | jq -r '.token')
//
//curl -s\
//    --header "Accept: application/vnd.docker.distribution.manifest.v2+json" \
//    --header "Authorization: Bearer $TOKEN" \
//    "https://registry-1.docker.io/v2/library/hello-world/manifests/latest" \
//    | jq

//TOKEN=$(curl -s "https://auth.docker.io/token?scope=repository:infoblox/dnstools:pull&service=registry.docker.io" | jq -r '.token')
//curl -s\
//    --header "Accept: application/vnd.docker.distribution.manifest.v2+json" \
//    --header "Authorization: Bearer $TOKEN" \
//    "https://registry-1.docker.io/v2/infoblox/dnstools/manifests/latest" \
//    | jq

//TOKEN=$(curl -s "https://auth.docker.io/token?scope=repository:calico/apiserver:pull&service=registry.docker.io" | jq -r '.token')
//curl -s\
//    --header "Accept: application/vnd.docker.distribution.manifest.v2+json" \
//    --header "Authorization: Bearer $TOKEN" \
//    "https://registry-1.docker.io/v2/calico/apiserver/manifests/v3.27.0" \
//    | jq

func main() {
	client, _ := reggie.NewClient("https://docker.io")
	req := client.NewRequest(reggie.GET, "/v2/hello-world/manifests/latest")
	resp, _ := client.Do(req)
	fmt.Println("Status Code:", resp.StatusCode())
	fmt.Println(string(resp.Body()))
}
