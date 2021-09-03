Simple utility to get schemas of all official Terraform Providers

How to Use
-----

1. Ensure you have `GOPATH` env variable set up

2. Run `make`

3. You'll see `schemas` directory with schemas and 'failure.txt' file with list of failed providers

4. Copy all json files from `schemas` directory to `/REPO_ROOT/terraform/model/providers/`
5. Copy providers.list to `/REPO_ROOT/terraform/model/`


Flags and Parameters
---------
### Filtering
- To filter providers:
  - `make providers ONLY="aws cloudflare"`
- To filter provisioners:
  - `make provisioners ONLY="local-exec file"`
- To filter backends:
  - `make backends ONLY="consul atlas"`

### Environment Variables - General
- `UPDATE_SKIP=1` - This skips the Git update process for all repositories ([See here](https://github.com/cageyv/terraform-metadata/blob/master/schemas-extractor/common.sh#L45))
- `UPDATE_PARALLEL=1` - Causes the update_or_clone function to be called in parrallel via & ([See here](https://github.com/cageyv/terraform-metadata/blob/master/schemas-extractor/common.sh#L70-L76))
- `GENERATE_PARALLEL=1` - Causes the schema generation to be called in parralel via & ([See here](https://github.com/cageyv/terraform-metadata/blob/master/schemas-extractor/build-providers.sh#L157-L163))
- `RESET_REPOS=1` - Script will force a git reset and git clean on the repository if it is not clear ([See here](https://github.com/cageyv/terraform-metadata/blob/master/schemas-extractor/build-providers.sh#L52-L60))
- `CHECK_PROVIDERS_NOFAIL=1` - Causes the check-providers script to not force exit if there are providers missing in the base file. ([See here](https://github.com/cageyv/terraform-metadata/blob/master/schemas-extractor/check-providers.sh#L19))
### Environment Variables - CI script only
- `KILL_CPU=1` - Causes the schema generation to run in parrallel via &. Same as `GENERATE_PARALLEL` above. ([See here](https://github.com/cageyv/terraform-metadata/blob/master/schemas-extractor/build-ci.sh#L145-L151))
