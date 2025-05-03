# TODO

1. Readthedocs
   - concurrency design
   - volumetrics
2. Code comments - spelling/grammar   
3. Test coverage
4. CI
   - govulncheck
   - SBOM
   - bench
   - labstack init
5. Consider https://github.com/OpenAPITools/openapi-generator
6. Instrumentation
7. Enable swagger UI (https://github.com/go-swagger/go-swagger)?
8. bin/imgpull is inserting "library", on "docker.io" pulls should it? (Would it work otherwise?)

CI
- server
  - oapi-codegen
  - test
  - vet
  - gocyclo?
  - coverprof?
  - vuln check
  - desktop (rename SERVER)
  - image
  - push

- chart
  - docs
  - package
  - push
