FROM golang:1.16-buster as setup

RUN apt-get update && apt-get install -y jq

RUN mkdir -p /workspace/terraform-metadata/schemas-extractor && mkdir -p /workspace/repos-for-extractor

COPY ./schemas-extractor/fetch-providers /workspace/terraform-metadata/schemas-extractor/fetch-providers
COPY ./schemas-extractor/update-clone-repos /workspace/terraform-metadata/schemas-extractor/update-clone-repos

WORKDIR /workspace/terraform-metadata/schemas-extractor

# We run these twice, when used with --cache-from this can greatly speed up the build process see the README.md for more details
RUN ./fetch-providers && ./update-clone-repos -b /workspace/repos-for-extractor

COPY ./schemas-extractor/common.sh /workspace/terraform-metadata/schemas-extractor/common.sh

#####################################
# Build Backends
#####################################
FROM setup as backends

ARG EXEC_RUNTIME
RUN ./update-clone-repos -b /workspace/repos-for-extractor -t "github.com/hashicorp/terraform"

COPY ./schemas-extractor/backends.json /workspace/terraform-metadata/schemas-extractor/backends.json
COPY ./schemas-extractor/template/backend /workspace/terraform-metadata/schemas-extractor/template/backend
COPY ./schemas-extractor/build-backends /workspace/terraform-metadata/schemas-extractor/build-backends
COPY ./terraform/model/backends /workspace/terraform-metadata/schemas-extractor/schemas/backends

RUN ./build-backends -b /workspace/repos-for-extractor

#####################################
# Build Provisioners
#####################################
FROM setup as provisioners

ARG EXEC_RUNTIME
RUN ./update-clone-repos -b /workspace/repos-for-extractor -t "github.com/hashicorp/terraform"

COPY ./schemas-extractor/template/provisioner /workspace/terraform-metadata/schemas-extractor/template/provisioner
COPY ./schemas-extractor/provisioners.json /workspace/terraform-metadata/schemas-extractor/provisioners.json
COPY ./schemas-extractor/build-provisioners /workspace/terraform-metadata/schemas-extractor/build-provisioners
COPY ./terraform/model/provisioners /workspace/terraform-metadata/schemas-extractor/schemas/provisioners

RUN ./build-provisioners -b /workspace/repos-for-extractor

#####################################
# Build Functions
#####################################
FROM setup as functions

ARG EXEC_RUNTIME
RUN ./update-clone-repos -b /workspace/repos-for-extractor -t "github.com/hashicorp/terraform"

COPY ./schemas-extractor/template/functions /workspace/terraform-metadata/schemas-extractor/template/functions
COPY ./schemas-extractor/functions.json /workspace/terraform-metadata/schemas-extractor/functions.json
COPY ./schemas-extractor/build-functions /workspace/terraform-metadata/schemas-extractor/build-functions

COPY ./terraform/model/functions.json /workspace/terraform-metadata/schemas-extractor/schemas/functions.json

RUN ./build-functions -b /workspace/repos-for-extractor

#####################################
# Build Providers
#####################################
FROM setup as providers

ARG EXEC_RUNTIME
COPY ./schemas-extractor/providers-base.json /workspace/terraform-metadata/schemas-extractor/providers-base.json
RUN ./fetch-providers && ./update-clone-repos -b /workspace/repos-for-extractor

COPY ./schemas-extractor/template/provider /workspace/terraform-metadata/schemas-extractor/template/provider
COPY ./schemas-extractor/build-providers /workspace/terraform-metadata/schemas-extractor/build-providers
COPY ./terraform/model/providers /workspace/terraform-metadata/schemas-extractor/schemas/providers

RUN ./build-providers -b /workspace/repos-for-extractor

#####################################
# Prep Output
#####################################
FROM scratch as output

COPY --from=backends /workspace/terraform-metadata/schemas-extractor/schemas/backends /terraform/model/backends
COPY --from=backends /workspace/terraform-metadata/schemas-extractor/schemas/backends.list /terraform/model/backends.list
COPY --from=backends /workspace/terraform-metadata/schemas-extractor/schemas/backends-failure.txt /terraform/model/backends-failure.txt

COPY --from=provisioners /workspace/terraform-metadata/schemas-extractor/schemas/provisioners /terraform/model/provisioners
COPY --from=provisioners /workspace/terraform-metadata/schemas-extractor/schemas/provisioners.list /terraform/model/provisioners.list
COPY --from=provisioners /workspace/terraform-metadata/schemas-extractor/schemas/provisioners-failure.txt /terraform/model/provisioners-failure.txt

COPY --from=functions /workspace/terraform-metadata/schemas-extractor/schemas/functions.json /terraform/model/functions.json
COPY --from=functions /workspace/terraform-metadata/schemas-extractor/schemas/functions-failure.txt /terraform/model/functions-failure.txt

COPY --from=providers /workspace/terraform-metadata/schemas-extractor/schemas/providers /terraform/model/providers
COPY --from=providers /workspace/terraform-metadata/schemas-extractor/schemas/providers.list /terraform/model/providers.list
COPY --from=providers /workspace/terraform-metadata/schemas-extractor/schemas/providers-failure.txt /terraform/model/providers-failure.txt
