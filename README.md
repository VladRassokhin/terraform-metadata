# Metadata for HashiCorp Terraform

### License

Apache 2.0 for everything here.

Terraform and most of providers source codes are licensed under MPL 2.0.
Since scripts in this repository are using them in terms of 'runtime' Apache 2.0 could be used for the results as well as scripts themselves.

The JetBrains Plugin reads from `~/.terraform.d/metadata-repo` (Linux, macOS) or `%APPDATA%/.terraform/metadata-repo` (Windows).
Just clone this repositry there:

```bash
mkdir -p "~/.terraform.d/metadata-repo"
git clone "https://github.com/ajb3ck/terraform-metadata" "~/.terraform.d/metadata-repo"
```
