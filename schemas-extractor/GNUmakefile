default: build

clean:
	rm -f repositories.list.full providers.list
	rm -rf schemas logs

build: schemas

schemas: providers.json
	@sh -c "'$(CURDIR)/build-providers.sh'"
	@sh -c "'$(CURDIR)/build-provisioners.sh'"
	@sh -c "'$(CURDIR)/build-backends.sh'"
	@sh -c "'$(CURDIR)/build-functions.sh'"

providers: providers.json
	@sh -c "'$(CURDIR)/build-providers.sh'"

provisioners: providers.json
	@sh -c "'$(CURDIR)/build-provisioners.sh'"

backends: providers.json
	@sh -c "'$(CURDIR)/build-backends.sh'"

functions: providers.json
	@sh -c "'$(CURDIR)/build-functions.sh'"

providers.list:
	@sh -c "'$(CURDIR)/fetch-providers.sh'"

providers.json: providers.list providers.base.json
	@sh -c "'$(CURDIR)/check-providers.sh'"
	cp providers.base.json providers.json


.PHONY: build
