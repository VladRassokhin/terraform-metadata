default: build

clean:
	rm -f repositories.list.full providers.list
	rm -rf schemas logs

build: schemas

providers-old: providers.json
	@sh -c "'$(CURDIR)/build-providers.sh'"

providers-new: providers.json
	@sh -c "'$(CURDIR)/build-providers-new.sh'"

schemas: providers.json providers-old
	@sh -c "'$(CURDIR)/build-provisioners.sh'"
	@sh -c "'$(CURDIR)/build-backends.sh'"
	@sh -c "'$(CURDIR)/build-functions.sh'"

providers.list:
	@sh -c "'$(CURDIR)/fetch-providers.sh'"

providers.json: providers.list providers.base.json
	@sh -c "'$(CURDIR)/check-providers.sh'"
	cp providers.base.json providers.json


.PHONY: build
