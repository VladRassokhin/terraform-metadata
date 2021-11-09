default: build

clean:
	rm -f repositories.list.full providers.registry.json
	rm -rf schemas logs

build: schemas

schemas: providers.json
	@sh -c "'$(CURDIR)/build-providers.sh'"
	@sh -c "'$(CURDIR)/build-provisioners.sh'"
	@sh -c "'$(CURDIR)/build-backends.sh'"
	@sh -c "'$(CURDIR)/build-functions.sh'"

providers.registry.json:
	@sh -c "'$(CURDIR)/fetch-providers-registry.sh'"

providers.json: providers.registry.json providers.base.json
	@sh -c "'$(CURDIR)/check-providers.sh'"
	cp providers.base.json providers.json


.PHONY: build
