

19-JAN
docker pull docker.io/hello-world:latest
latest: Pulling from library/hello-world
Digest: sha256:4bd78111b6914a99dbc560e6a20eab57ff6655aea4a80c50b0c5491968cbc2e6
Status: Downloaded newer image for hello-world:latest
docker.io/library/hello-world:latest

crane pull  docker.io/hello-world:latest hello-world-latest-19jan.tar --cache_path /tmp
2024/01/19 15:32:21 Layer sha256:c1ec31eb59444d78df06a974d155e597c894ab4cda84f08294145e845394988e not found (compressed) in cache, getting

tar -xvf hello-world-latest-19jan.tar manifest.json --to-stdout |jq

manifest.json
[
  {
    "Config": "sha256:d2c94e258dcb3c5ac2798d32e1249e42ef01cba4841c2234249495f87264ac5a",
    "RepoTags": [
      "docker.io/hello-world:latest"
    ],
    "Layers": [
      "c1ec31eb59444d78df06a974d155e597c894ab4cda84f08294145e845394988e.tar.gz"
    ]
  }
]

tar -xf hello-world-latest-19jan.tar -C images/library/hello-world/latest/

find images/library/hello-world/latest/


run my server
curl -X GET http://localhost:8080/v2/hello-world/manifests/latest -v

Docker-Content-Digest: sha256:e6987062f893ebea17804cc9234783f887df8aad1b39a6bba3e7b83211cd1d80 <<<<<<<<<<<<<<< DOES NOT MATCH ABOVE

run containerd

sudo ./crictl pull localhost:8080/hello-world:latest


containerd says:

E0119 15:44:15.415452  799335 remote_image.go:180] "PullImage from image service failed" err="rpc error:
code = NotFound desc = failed to pull and unpack image \"localhost:8080/hello-world:latest\": failed to copy:
httpReadSeeker: failed open: could not fetch content descriptor sha256:e6987062f893ebea17804cc9234783f887df8aad1b39a6bba3e7b83211cd1d80
() from remote: not found" image="localhost:8080/hello-world:latest"
FATA[0000] pulling image: rpc error: code = NotFound desc = failed to pull and unpack
image "localhost:8080/hello-world:latest": failed to copy: httpReadSeeker: failed open: could
not fetch content descriptor sha256:e6987062f893ebea17804cc9234783f887df8aad1b39a6bba3e7b83211cd1d80 () from remote: not found 




LOGS IN MYU SERVER
containerd <<<<<<<<<<<<<< why is it getting the manifest digest e6987062 as a blob!!!?

"method":"HEAD","uri":"/v2/hello-world/manifests/latest","user_agent":"containerd/2.0.0-beta.0+unknown","status":200
"method":"GET","uri":"/v2/hello-world/blobs/sha256:e6987062f893ebea17804cc9234783f887df8aad1b39a6bba3e7b83211cd1d80","user_agent":"containerd/2.0.0-beta.0+unknown","status":404

"ociregistry.go","line":"124","message":"HEAD manifest - org: library, image: hello-world, ref: latest"}
"ociregistry.go","line":"141","message":"found manifest - /home/eace/projects/ociregistry/images/library/hello-world/latest/manifest.json"}
"ociregistry.go","line":"169","message":"get layer - c1ec31eb59444d78df06a974d155e597c894ab4cda84f08294145e845394988e.tar.gz"}
"ociregistry.go","line":"174","message":"found layer - /home/eace/projects/ociregistry/images/library/hello-world/latest/c1ec31eb59444d78df06a974d155e597c894ab4cda84f08294145e845394988e.tar.gz"}
"ociregistry.go","line":"197","message":"computed digest for ref latest = sha256:e6987062f893ebea17804cc9234783f887df8aad1b39a6bba3e7b83211cd1d80 (cnt: 424 / mblen:424)"}
"ociregistry.go","line":"69" ,"message":"get blob - org: library, image: hello-world, digest: sha256:e6987062f893ebea17804cc9234783f887df8aad1b39a6bba3e7b83211cd1d80"}



> curl https://auth.docker.io/token?scope=repository::pull&service=registry.docker.i
< {"token":"eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsIng1YyI6WyJNSUlDK1RDQ0FwK2dBd0lCQWdJQkFEQUtCZ2dxaGtqT1BRUURBakJHTVVRd1FnWURWUVFERXp0U1RVbEdPbEZNUmpRNlEwZFFNenBSTWtWYU9sRklSRUk2VkVkRlZUcFZTRlZNT2taTVZqUTZSMGRXV2pwQk5WUkhPbFJMTkZNNlVVeElTVEFlRncweU16QXhNRFl3TkRJM05EUmFGdzB5TkRBeE1qWXdOREkzTkRSYU1FWXhSREJDQmdOVkJBTVRPME5EVlVZNlNqVkhOanBGUTFORU9rTldSRWM2VkRkTU1qcEtXa1pST2xOTk0wUTZXRmxQTkRwV04wTkhPa2RHVjBJNldsbzFOam8wVlVSRE1JSUJJakFOQmdrcWhraUc5dzBCQVFFRkFBT0NBUThBTUlJQkNnS0NBUUVBek4wYjBqN1V5L2FzallYV2gyZzNxbzZKaE9rQWpYV0FVQmNzSHU2aFlaUkZMOXZlODEzVEI0Y2w4UWt4Q0k0Y1VnR0duR1dYVnhIMnU1dkV0eFNPcVdCcnhTTnJoU01qL1ZPKzYvaVkrOG1GRmEwR2J5czF3VDVjNlY5cWROaERiVGNwQXVYSjFSNGJLdSt1VGpVS0VIYXlqSFI5TFBEeUdnUC9ubUFadk5PWEdtclNTSkZJNnhFNmY3QS8rOVptcWgyVlRaQlc0cXduSnF0cnNJM2NveDNQczMwS2MrYUh3V3VZdk5RdFNBdytqVXhDVVFoRWZGa0lKSzh6OVdsL1FjdE9EcEdUeXNtVHBjNzZaVEdKWWtnaGhGTFJEMmJQTlFEOEU1ZWdKa2RQOXhpaW5sVGx3MjBxWlhVRmlqdWFBcndOR0xJbUJEWE0wWlI1YzVtU3Z3SURBUUFCbzRHeU1JR3ZNQTRHQTFVZER3RUIvd1FFQXdJSGdEQVBCZ05WSFNVRUNEQUdCZ1JWSFNVQU1FUUdBMVVkRGdROUJEdERRMVZHT2tvMVJ6WTZSVU5UUkRwRFZrUkhPbFEzVERJNlNscEdVVHBUVFRORU9saFpUelE2VmpkRFJ6cEhSbGRDT2xwYU5UWTZORlZFUXpCR0JnTlZIU01FUHpBOWdEdFNUVWxHT2xGTVJqUTZRMGRRTXpwUk1rVmFPbEZJUkVJNlZFZEZWVHBWU0ZWTU9rWk1WalE2UjBkV1dqcEJOVlJIT2xSTE5GTTZVVXhJU1RBS0JnZ3Foa2pPUFFRREFnTklBREJGQWlFQW1RNHhsQXZXVlArTy9hNlhDU05pYUFYRU1Bb1RQVFRYRWJYMks2RVU4ZTBDSUg0QTAwSVhtUndjdGtEOHlYNzdkTVoyK0pEY1FGdDFxRktMZFR5SnVzT1UiXX0.eyJhY2Nlc3MiOltdLCJhdWQiOiIiLCJleHAiOjE3MDU2OTgyODAsImlhdCI6MTcwNTY5Nzk4MCwiaXNzIjoiYXV0aC5kb2NrZXIuaW8iLCJqdGkiOiJkY2tyX2p0aV9jVXVvd085VUhEWmFkMzI2aEdIVkFQUVV0MUk9IiwibmJmIjoxNzA1Njk3NjgwLCJzdWIiOiIifQ.ssvCIuLMka2dDL846dFrCsAZFE02u-QB0QuekMs8AZ1iNkppLgfrZr5eiyVNfrGosDjhCsXf0GYF4YZRdBkXmZs_twaSFQK5qMC9irvXjA1sOl0sVRcv9Tx3DJ_4ooyTKWLLsMXuN8DjHnzyen6tJQ2HLJ1Y0m6YaH79K6VtB_QpKxhfnpzG87zcDhrEWG_UJVdK0CNECRQw6HQw7PbaEZJRiEAWs3KtlwSEVf6NjoJg62h2S9E1TwbNoU9D_XrNq1wKXj6wBMTMFOlH1bYMhqQq6-0pTo9rTqGC5CIQvCPKFqn4AuLrZfoSmb8G8J0QzLuy6wayvFGCg6dYWMnaCg","access_token":"eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsIng1YyI6WyJNSUlDK1RDQ0FwK2dBd0lCQWdJQkFEQUtCZ2dxaGtqT1BRUURBakJHTVVRd1FnWURWUVFERXp0U1RVbEdPbEZNUmpRNlEwZFFNenBSTWtWYU9sRklSRUk2VkVkRlZUcFZTRlZNT2taTVZqUTZSMGRXV2pwQk5WUkhPbFJMTkZNNlVVeElTVEFlRncweU16QXhNRFl3TkRJM05EUmFGdzB5TkRBeE1qWXdOREkzTkRSYU1FWXhSREJDQmdOVkJBTVRPME5EVlVZNlNqVkhOanBGUTFORU9rTldSRWM2VkRkTU1qcEtXa1pST2xOTk0wUTZXRmxQTkRwV04wTkhPa2RHVjBJNldsbzFOam8wVlVSRE1JSUJJakFOQmdrcWhraUc5dzBCQVFFRkFBT0NBUThBTUlJQkNnS0NBUUVBek4wYjBqN1V5L2FzallYV2gyZzNxbzZKaE9rQWpYV0FVQmNzSHU2aFlaUkZMOXZlODEzVEI0Y2w4UWt4Q0k0Y1VnR0duR1dYVnhIMnU1dkV0eFNPcVdCcnhTTnJoU01qL1ZPKzYvaVkrOG1GRmEwR2J5czF3VDVjNlY5cWROaERiVGNwQXVYSjFSNGJLdSt1VGpVS0VIYXlqSFI5TFBEeUdnUC9ubUFadk5PWEdtclNTSkZJNnhFNmY3QS8rOVptcWgyVlRaQlc0cXduSnF0cnNJM2NveDNQczMwS2MrYUh3V3VZdk5RdFNBdytqVXhDVVFoRWZGa0lKSzh6OVdsL1FjdE9EcEdUeXNtVHBjNzZaVEdKWWtnaGhGTFJEMmJQTlFEOEU1ZWdKa2RQOXhpaW5sVGx3MjBxWlhVRmlqdWFBcndOR0xJbUJEWE0wWlI1YzVtU3Z3SURBUUFCbzRHeU1JR3ZNQTRHQTFVZER3RUIvd1FFQXdJSGdEQVBCZ05WSFNVRUNEQUdCZ1JWSFNVQU1FUUdBMVVkRGdROUJEdERRMVZHT2tvMVJ6WTZSVU5UUkRwRFZrUkhPbFEzVERJNlNscEdVVHBUVFRORU9saFpUelE2VmpkRFJ6cEhSbGRDT2xwYU5UWTZORlZFUXpCR0JnTlZIU01FUHpBOWdEdFNUVWxHT2xGTVJqUTZRMGRRTXpwUk1rVmFPbEZJUkVJNlZFZEZWVHBWU0ZWTU9rWk1WalE2UjBkV1dqcEJOVlJIT2xSTE5GTTZVVXhJU1RBS0JnZ3Foa2pPUFFRREFnTklBREJGQWlFQW1RNHhsQXZXVlArTy9hNlhDU05pYUFYRU1Bb1RQVFRYRWJYMks2RVU4ZTBDSUg0QTAwSVhtUndjdGtEOHlYNzdkTVoyK0pEY1FGdDFxRktMZFR5SnVzT1UiXX0.eyJhY2Nlc3MiOltdLCJhdWQiOiIiLCJleHAiOjE3MDU2OTgyODAsImlhdCI6MTcwNTY5Nzk4MCwiaXNzIjoiYXV0aC5kb2NrZXIuaW8iLCJqdGkiOiJkY2tyX2p0aV9jVXVvd085VUhEWmFkMzI2aEdIVkFQUVV0MUk9IiwibmJmIjoxNzA1Njk3NjgwLCJzdWIiOiIifQ.ssvCIuLMka2dDL846dFrCsAZFE02u-QB0QuekMs8AZ1iNkppLgfrZr5eiyVNfrGosDjhCsXf0GYF4YZRdBkXmZs_twaSFQK5qMC9irvXjA1sOl0sVRcv9Tx3DJ_4ooyTKWLLsMXuN8DjHnzyen6tJQ2HLJ1Y0m6YaH79K6VtB_QpKxhfnpzG87zcDhrEWG_UJVdK0CNECRQw6HQw7PbaEZJRiEAWs3KtlwSEVf6NjoJg62h2S9E1TwbNoU9D_XrNq1wKXj6wBMTMFOlH1bYMhqQq6-0pTo9rTqGC5CIQvCPKFqn4AuLrZfoSmb8G8J0QzLuy6wayvFGCg6dYWMnaCg","expires_in":300,"issued_at":"2024-01-19T20:59:40.042383011Z"}

export TOKEN="eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsIng1YyI6WyJNSUlDK1RDQ0FwK2dBd0lCQWdJQkFEQUtCZ2dxaGtqT1BRUURBakJHTVVRd1FnWURWUVFERXp0U1RVbEdPbEZNUmpRNlEwZFFNenBSTWtWYU9sRklSRUk2VkVkRlZUcFZTRlZNT2taTVZqUTZSMGRXV2pwQk5WUkhPbFJMTkZNNlVVeElTVEFlRncweU16QXhNRFl3TkRJM05EUmFGdzB5TkRBeE1qWXdOREkzTkRSYU1FWXhSREJDQmdOVkJBTVRPME5EVlVZNlNqVkhOanBGUTFORU9rTldSRWM2VkRkTU1qcEtXa1pST2xOTk0wUTZXRmxQTkRwV04wTkhPa2RHVjBJNldsbzFOam8wVlVSRE1JSUJJakFOQmdrcWhraUc5dzBCQVFFRkFBT0NBUThBTUlJQkNnS0NBUUVBek4wYjBqN1V5L2FzallYV2gyZzNxbzZKaE9rQWpYV0FVQmNzSHU2aFlaUkZMOXZlODEzVEI0Y2w4UWt4Q0k0Y1VnR0duR1dYVnhIMnU1dkV0eFNPcVdCcnhTTnJoU01qL1ZPKzYvaVkrOG1GRmEwR2J5czF3VDVjNlY5cWROaERiVGNwQXVYSjFSNGJLdSt1VGpVS0VIYXlqSFI5TFBEeUdnUC9ubUFadk5PWEdtclNTSkZJNnhFNmY3QS8rOVptcWgyVlRaQlc0cXduSnF0cnNJM2NveDNQczMwS2MrYUh3V3VZdk5RdFNBdytqVXhDVVFoRWZGa0lKSzh6OVdsL1FjdE9EcEdUeXNtVHBjNzZaVEdKWWtnaGhGTFJEMmJQTlFEOEU1ZWdKa2RQOXhpaW5sVGx3MjBxWlhVRmlqdWFBcndOR0xJbUJEWE0wWlI1YzVtU3Z3SURBUUFCbzRHeU1JR3ZNQTRHQTFVZER3RUIvd1FFQXdJSGdEQVBCZ05WSFNVRUNEQUdCZ1JWSFNVQU1FUUdBMVVkRGdROUJEdERRMVZHT2tvMVJ6WTZSVU5UUkRwRFZrUkhPbFEzVERJNlNscEdVVHBUVFRORU9saFpUelE2VmpkRFJ6cEhSbGRDT2xwYU5UWTZORlZFUXpCR0JnTlZIU01FUHpBOWdEdFNUVWxHT2xGTVJqUTZRMGRRTXpwUk1rVmFPbEZJUkVJNlZFZEZWVHBWU0ZWTU9rWk1WalE2UjBkV1dqcEJOVlJIT2xSTE5GTTZVVXhJU1RBS0JnZ3Foa2pPUFFRREFnTklBREJGQWlFQW1RNHhsQXZXVlArTy9hNlhDU05pYUFYRU1Bb1RQVFRYRWJYMks2RVU4ZTBDSUg0QTAwSVhtUndjdGtEOHlYNzdkTVoyK0pEY1FGdDFxRktMZFR5SnVzT1UiXX0.eyJhY2Nlc3MiOltdLCJhdWQiOiIiLCJleHAiOjE3MDU2OTgyODAsImlhdCI6MTcwNTY5Nzk4MCwiaXNzIjoiYXV0aC5kb2NrZXIuaW8iLCJqdGkiOiJkY2tyX2p0aV9jVXVvd085VUhEWmFkMzI2aEdIVkFQUVV0MUk9IiwibmJmIjoxNzA1Njk3NjgwLCJzdWIiOiIifQ.ssvCIuLMka2dDL846dFrCsAZFE02u-QB0QuekMs8AZ1iNkppLgfrZr5eiyVNfrGosDjhCsXf0GYF4YZRdBkXmZs_twaSFQK5qMC9irvXjA1sOl0sVRcv9Tx3DJ_4ooyTKWLLsMXuN8DjHnzyen6tJQ2HLJ1Y0m6YaH79K6VtB_QpKxhfnpzG87zcDhrEWG_UJVdK0CNECRQw6HQw7PbaEZJRiEAWs3KtlwSEVf6NjoJg62h2S9E1TwbNoU9D_XrNq1wKXj6wBMTMFOlH1bYMhqQq6-0pTo9rTqGC5CIQvCPKFqn4AuLrZfoSmb8G8J0QzLuy6wayvFGCg6dYWMnaCg"

curl -v\
 -H "Accept: application/vnd.docker.distribution.manifest.v2+json"\
 -H "Authorization: Bearer $TOKEN"\
 https://registry-1.docker.io/v2/hello-world/manifests/latest

FAILS




different answer
----------------
curl \
    --silent \
    "https://auth.docker.io/token?scope=repository:library/hello-world:pull&service=registry.docker.io" \
    | jq -r '.token'


export  TOKEN="eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsIng1YyI6WyJNSUlDK1RDQ0FwK2dBd0lCQWdJQkFEQUtCZ2dxaGtqT1BRUURBakJHTVVRd1FnWURWUVFERXp0U1RVbEdPbEZNUmpRNlEwZFFNenBSTWtWYU9sRklSRUk2VkVkRlZUcFZTRlZNT2taTVZqUTZSMGRXV2pwQk5WUkhPbFJMTkZNNlVVeElTVEFlRncweU16QXhNRFl3TkRJM05EUmFGdzB5TkRBeE1qWXdOREkzTkRSYU1FWXhSREJDQmdOVkJBTVRPME5EVlVZNlNqVkhOanBGUTFORU9rTldSRWM2VkRkTU1qcEtXa1pST2xOTk0wUTZXRmxQTkRwV04wTkhPa2RHVjBJNldsbzFOam8wVlVSRE1JSUJJakFOQmdrcWhraUc5dzBCQVFFRkFBT0NBUThBTUlJQkNnS0NBUUVBek4wYjBqN1V5L2FzallYV2gyZzNxbzZKaE9rQWpYV0FVQmNzSHU2aFlaUkZMOXZlODEzVEI0Y2w4UWt4Q0k0Y1VnR0duR1dYVnhIMnU1dkV0eFNPcVdCcnhTTnJoU01qL1ZPKzYvaVkrOG1GRmEwR2J5czF3VDVjNlY5cWROaERiVGNwQXVYSjFSNGJLdSt1VGpVS0VIYXlqSFI5TFBEeUdnUC9ubUFadk5PWEdtclNTSkZJNnhFNmY3QS8rOVptcWgyVlRaQlc0cXduSnF0cnNJM2NveDNQczMwS2MrYUh3V3VZdk5RdFNBdytqVXhDVVFoRWZGa0lKSzh6OVdsL1FjdE9EcEdUeXNtVHBjNzZaVEdKWWtnaGhGTFJEMmJQTlFEOEU1ZWdKa2RQOXhpaW5sVGx3MjBxWlhVRmlqdWFBcndOR0xJbUJEWE0wWlI1YzVtU3Z3SURBUUFCbzRHeU1JR3ZNQTRHQTFVZER3RUIvd1FFQXdJSGdEQVBCZ05WSFNVRUNEQUdCZ1JWSFNVQU1FUUdBMVVkRGdROUJEdERRMVZHT2tvMVJ6WTZSVU5UUkRwRFZrUkhPbFEzVERJNlNscEdVVHBUVFRORU9saFpUelE2VmpkRFJ6cEhSbGRDT2xwYU5UWTZORlZFUXpCR0JnTlZIU01FUHpBOWdEdFNUVWxHT2xGTVJqUTZRMGRRTXpwUk1rVmFPbEZJUkVJNlZFZEZWVHBWU0ZWTU9rWk1WalE2UjBkV1dqcEJOVlJIT2xSTE5GTTZVVXhJU1RBS0JnZ3Foa2pPUFFRREFnTklBREJGQWlFQW1RNHhsQXZXVlArTy9hNlhDU05pYUFYRU1Bb1RQVFRYRWJYMks2RVU4ZTBDSUg0QTAwSVhtUndjdGtEOHlYNzdkTVoyK0pEY1FGdDFxRktMZFR5SnVzT1UiXX0.eyJhY2Nlc3MiOlt7InR5cGUiOiJyZXBvc2l0b3J5IiwibmFtZSI6ImxpYnJhcnkvaGVsbG8td29ybGQiLCJhY3Rpb25zIjpbInB1bGwiXSwicGFyYW1ldGVycyI6eyJwdWxsX2xpbWl0IjoiMTAwIiwicHVsbF9saW1pdF9pbnRlcnZhbCI6IjIxNjAwIn19XSwiYXVkIjoicmVnaXN0cnkuZG9ja2VyLmlvIiwiZXhwIjoxNzA1Njk4NjkxLCJpYXQiOjE3MDU2OTgzOTEsImlzcyI6ImF1dGguZG9ja2VyLmlvIiwianRpIjoiZGNrcl9qdGlfUUxZaWF6SU5DZlg0OUVBaWpHQVVEd1ZQcUNjPSIsIm5iZiI6MTcwNTY5ODA5MSwic3ViIjoiIn0.F8vXySkEdbGNcfLUeVPlARU-agMX5UvHD0RYwZ_AwLbrznfYW7mY_ofnaE4aePOA0w3HjR1xaBWe1Bt7VDIQ6jJ--vQwy-qWqXhQ-w4jqV969JRmO9ngL4AOwGiZBsG89yu08B_idPflJ29vOGiQMGTwwObYkoKBVn8daDdVs27kL-8S1CikBXYux0cOWQMHo3oaZSNKLJgj1ELNmJwUbHzFDongxlXg3vmPrS8ytGJtyKkyiDElf5Xnp0hGq-5ilcidi1L_1TUhBnHAPefCL9Dhnm3N077KWlvd_k9ez6ykWYBjNF7mEaz2RUtm_z5YDG-kt6QiaCyXFV1jB3XySw"


curl \
    --header "Accept: application/vnd.docker.distribution.manifest.v2+json" \
    --header "Authorization: Bearer $TOKEN" \
    "https://registry-1.docker.io/v2/library/hello-world/manifests/latest" \
    | jq

HEADERS:
* TLSv1.2 (IN), TLS header, Supplemental data (23):
* Mark bundle as not supporting multiuse
< HTTP/1.1 200 OK
< content-length: 9125
< content-type: application/vnd.oci.image.index.v1+json
< docker-content-digest: sha256:4bd78111b6914a99dbc560e6a20eab57ff6655aea4a80c50b0c5491968cbc2e6
< docker-distribution-api-version: registry/2.0
< etag: "sha256:4bd78111b6914a99dbc560e6a20eab57ff6655aea4a80c50b0c5491968cbc2e6"
< date: Fri, 19 Jan 2024 21:10:26 GMT
< strict-transport-security: max-age=31536000
< ratelimit-limit: 100;w=21600
< ratelimit-remaining: 96;w=21600
< docker-ratelimit-source: 2601:14e:8000:728c:773a:d015:17c7:1eae


SO THIS EXPLAINS WHY THE SHA IS DIFFERENT BECAUSE I'M RETURNING A DIFFERENT MANIFEST
SEE BELOW "Finally a GET request to retrieve the container config, using the digest we received in step 2"

SO IT ***MIGHT*** be as simple as handling a blob request that returns a manifest - not a layer!!

{
  "manifests": [
    {
      "annotations": {
        "org.opencontainers.image.revision": "3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee",
        "org.opencontainers.image.source": "https://github.com/docker-library/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:amd64/hello-world",
        "org.opencontainers.image.url": "https://hub.docker.com/_/hello-world",
        "org.opencontainers.image.version": "linux"
      },
      "digest": "sha256:e2fc4e5012d16e7fe466f5291c476431beaa1f9b90a5c2125b493ed28e2aba57",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "amd64",
        "os": "linux"
      },
      "size": 861
    },
    {
      "annotations": {
        "vnd.docker.reference.digest": "sha256:e2fc4e5012d16e7fe466f5291c476431beaa1f9b90a5c2125b493ed28e2aba57",
        "vnd.docker.reference.type": "attestation-manifest"
      },
      "digest": "sha256:579b3724a7b189f6dca599a46f16d801a43d5def185de0b7bcd5fb9d1e312c27",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "unknown",
        "os": "unknown"
      },
      "size": 837
    },
    {
      "annotations": {
        "org.opencontainers.image.revision": "3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee",
        "org.opencontainers.image.source": "https://github.com/docker-library/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:arm32v5/hello-world",
        "org.opencontainers.image.url": "https://hub.docker.com/_/hello-world",
        "org.opencontainers.image.version": "linux"
      },
      "digest": "sha256:c2d891e5c2fb4c723efb72b064be3351189f62222bd3681ce7e57f2a1527362c",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "arm",
        "os": "linux",
        "variant": "v5"
      },
      "size": 863
    },
    {
      "annotations": {
        "vnd.docker.reference.digest": "sha256:c2d891e5c2fb4c723efb72b064be3351189f62222bd3681ce7e57f2a1527362c",
        "vnd.docker.reference.type": "attestation-manifest"
      },
      "digest": "sha256:6901d6a88eee6e90f0baa62b020bb61c4f13194cbcd9bf568ab66e8cc3f940dd",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "unknown",
        "os": "unknown"
      },
      "size": 566
    },
    {
      "annotations": {
        "org.opencontainers.image.revision": "3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee",
        "org.opencontainers.image.source": "https://github.com/docker-library/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:arm32v7/hello-world",
        "org.opencontainers.image.url": "https://hub.docker.com/_/hello-world",
        "org.opencontainers.image.version": "linux"
      },
      "digest": "sha256:20aea1c63c90d5e117db787c9fe1a8cd0ad98bedb5fd711273ffe05c084ff18a",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "arm",
        "os": "linux",
        "variant": "v7"
      },
      "size": 863
    },
    {
      "annotations": {
        "vnd.docker.reference.digest": "sha256:20aea1c63c90d5e117db787c9fe1a8cd0ad98bedb5fd711273ffe05c084ff18a",
        "vnd.docker.reference.type": "attestation-manifest"
      },
      "digest": "sha256:70304c314d8a61ba1b36518624bb00bfff8d4b6016153792042de43f0453ca61",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "unknown",
        "os": "unknown"
      },
      "size": 837
    },
    {
      "annotations": {
        "org.opencontainers.image.revision": "3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee",
        "org.opencontainers.image.source": "https://github.com/docker-library/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:arm64v8/hello-world",
        "org.opencontainers.image.url": "https://hub.docker.com/_/hello-world",
        "org.opencontainers.image.version": "linux"
      },
      "digest": "sha256:2d4e459f4ecb5329407ae3e47cbc107a2fbace221354ca75960af4c047b3cb13",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "arm64",
        "os": "linux",
        "variant": "v8"
      },
      "size": 863
    },
    {
      "annotations": {
        "vnd.docker.reference.digest": "sha256:2d4e459f4ecb5329407ae3e47cbc107a2fbace221354ca75960af4c047b3cb13",
        "vnd.docker.reference.type": "attestation-manifest"
      },
      "digest": "sha256:1f11fbd1720fcae3e402fc3eecb7d57c67023d2d1e11becc99ad9c7fe97d65ca",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "unknown",
        "os": "unknown"
      },
      "size": 837
    },
    {
      "annotations": {
        "org.opencontainers.image.revision": "3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee",
        "org.opencontainers.image.source": "https://github.com/docker-library/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:i386/hello-world",
        "org.opencontainers.image.url": "https://hub.docker.com/_/hello-world",
        "org.opencontainers.image.version": "linux"
      },
      "digest": "sha256:dbbd3cf666311ad526fad9d1746177469268f32fd91b371df2ebd1c84eb22f23",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "386",
        "os": "linux"
      },
      "size": 860
    },
    {
      "annotations": {
        "vnd.docker.reference.digest": "sha256:dbbd3cf666311ad526fad9d1746177469268f32fd91b371df2ebd1c84eb22f23",
        "vnd.docker.reference.type": "attestation-manifest"
      },
      "digest": "sha256:18b1c92de36d42c75440c6fd6b25605cc91709d176faaccca8afe58b317bc33a",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "unknown",
        "os": "unknown"
      },
      "size": 566
    },
    {
      "annotations": {
        "org.opencontainers.image.revision": "3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee",
        "org.opencontainers.image.source": "https://github.com/docker-library/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:mips64le/hello-world",
        "org.opencontainers.image.url": "https://hub.docker.com/_/hello-world",
        "org.opencontainers.image.version": "linux"
      },
      "digest": "sha256:c19784034d46da48550487c5c44639f5f92d48be7b9baf4d67b5377a454d92af",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "mips64le",
        "os": "linux"
      },
      "size": 864
    },
    {
      "annotations": {
        "vnd.docker.reference.digest": "sha256:c19784034d46da48550487c5c44639f5f92d48be7b9baf4d67b5377a454d92af",
        "vnd.docker.reference.type": "attestation-manifest"
      },
      "digest": "sha256:951bcd144ddccd1ee902dc180b435faabaaa6a8747e70cbc893f2dca16badb94",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "unknown",
        "os": "unknown"
      },
      "size": 566
    },
    {
      "annotations": {
        "org.opencontainers.image.revision": "3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee",
        "org.opencontainers.image.source": "https://github.com/docker-library/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:ppc64le/hello-world",
        "org.opencontainers.image.url": "https://hub.docker.com/_/hello-world",
        "org.opencontainers.image.version": "linux"
      },
      "digest": "sha256:f0c95f1ebb50c9b0b3e3416fb9dd4d1d197386a076c464cceea3d1f94c321b8f",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "ppc64le",
        "os": "linux"
      },
      "size": 863
    },
    {
      "annotations": {
        "vnd.docker.reference.digest": "sha256:f0c95f1ebb50c9b0b3e3416fb9dd4d1d197386a076c464cceea3d1f94c321b8f",
        "vnd.docker.reference.type": "attestation-manifest"
      },
      "digest": "sha256:838d191bca398e46cddebc48e816da83b0389d4ed2d64f408d618521b8fd1a57",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "unknown",
        "os": "unknown"
      },
      "size": 837
    },
    {
      "annotations": {
        "org.opencontainers.image.revision": "3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee",
        "org.opencontainers.image.source": "https://github.com/docker-library/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:riscv64/hello-world",
        "org.opencontainers.image.url": "https://hub.docker.com/_/hello-world",
        "org.opencontainers.image.version": "linux"
      },
      "digest": "sha256:8d064a6fc27fd5e97fa8225994a1addd872396236367745bea30c92d6c032fa3",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "riscv64",
        "os": "linux"
      },
      "size": 863
    },
    {
      "annotations": {
        "vnd.docker.reference.digest": "sha256:8d064a6fc27fd5e97fa8225994a1addd872396236367745bea30c92d6c032fa3",
        "vnd.docker.reference.type": "attestation-manifest"
      },
      "digest": "sha256:48147407c4594e45b7c3f0be1019bb0f44d78d7f037ce63e0e3da75b256f849e",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "unknown",
        "os": "unknown"
      },
      "size": 837
    },
    {
      "annotations": {
        "org.opencontainers.image.revision": "3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee",
        "org.opencontainers.image.source": "https://github.com/docker-library/hello-world.git#3fb6ebca4163bf5b9cc496ac3e8f11cb1e754aee:s390x/hello-world",
        "org.opencontainers.image.url": "https://hub.docker.com/_/hello-world",
        "org.opencontainers.image.version": "linux"
      },
      "digest": "sha256:65f4b0d1802589b418bb6774d85de3d1a11d5bd971ee73cb8569504d928bb5d9",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "s390x",
        "os": "linux"
      },
      "size": 861
    },
    {
      "annotations": {
        "vnd.docker.reference.digest": "sha256:65f4b0d1802589b418bb6774d85de3d1a11d5bd971ee73cb8569504d928bb5d9",
        "vnd.docker.reference.type": "attestation-manifest"
      },
      "digest": "sha256:50f420e8710676da03668e446f1f51097b745e3e2c9807b018e569d26d4f65f7",
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "platform": {
        "architecture": "unknown",
        "os": "unknown"
      },
      "size": 837
    },
    {
      "digest": "sha256:06a89fb00097b4398c84a7ddb00b3d9fd220780d1a22ae10d0d40ba00d6d98a0",
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "platform": {
        "architecture": "amd64",
        "os": "windows",
        "os.version": "10.0.20348.2227"
      },
      "size": 946
    },
    {
      "digest": "sha256:741e985f49fc0777c7cb5d3a0018a303e6180b4b4a55b782906b716b24c8183d",
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "platform": {
        "architecture": "amd64",
        "os": "windows",
        "os.version": "10.0.17763.5329"
      },
      "size": 946
    }
  ],
  "mediaType": "application/vnd.oci.image.index.v1+json",
  "schemaVersion": 2
}










NOTICE THE BLOB REQEST FOR THE CONTAINER CONFIG!!!!! *****************************************************************************************

https://stackoverflow.com/questions/55386202/how-can-i-use-the-docker-registry-api-to-pull-information-about-a-container-get

A GET request to auth.docker.io to get a token

curl "https://auth.docker.io/token?scope=repository:<image>:pull&service=registry.docker.io"

In this case image could be something like nginx or docker - basically whatever image you're looking up. This REST call returns a token to use in subsequent requests.

A GET request to retrieve the manifest listings

curl -H "Accept: application/vnd.docker.distribution.manifest.v2+json"
-H "Authorization: Bearer <token-from-step-1>"
"https://registry-1.docker.io/v2/<image>/manifests/<tag>"

Here image is the same as in Step 1, and tag could be something like latest. This call returns some JSON; the key is that we need to extract the value at .config.digest. This is the digest string that we use in the final request.

Finally a GET request to retrieve the container config, using the digest we received in step 2

curl -H "Accept: application/vnd.docker.distribution.manifest.v2+json"
-H "Authorization: Bearer <token-from-step-1>"
"https://registry-1.docker.io/v2/<image>/blobs/<digest-from-step-2>"




=======
FIDDLER
=======
This operation changes the proxy settings for your active network connection. When you
switch off the toggle in the main interface or close the application, the proxy is removed.
However, there may be situations where the proxy is not removed successfully, which could
result in the loss of internet connection. If this happens, go to Settings and search for
“proxy”. Go to Manual Proxy Setup, press “Edit” and disable the proxy.


sudo /home/eace/projects/containerd/esace/containerd\
 --config /home/eace/projects/containerd/esace/config.toml\
 --address /run/containerdtest/containerd.sock
 
     






