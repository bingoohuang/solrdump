SHELL = /bin/bash
TARGETS = solrdump
PKGNAME = solrdump

all: $(TARGETS)

$(TARGETS): %: main.go
	go build -o $@ $< 

clean:
	rm -f $(TARGETS)
	rm -f $(PKGNAME)_*deb
	rm -f $(PKGNAME)-*rpm
	rm -rf _packaging/deb/$(PKGNAME)/usr

imports:
	goimports -w .

deb: $(TARGETS)
	mkdir -p _packaging/deb/$(PKGNAME)/usr/sbin
	cp $(TARGETS) _packaging/deb/$(PKGNAME)/usr/sbin
	cd _packaging/deb && fakeroot dpkg-deb --build $(PKGNAME) .
	mv _packaging/deb/$(PKGNAME)_*.deb .

rpm: $(TARGETS)
	mkdir -p $(HOME)/rpmbuild/{BUILD,SOURCES,SPECS,RPMS}
	cp ./packaging/rpm/$(PKGNAME).spec $(HOME)/rpmbuild/SPECS
	cp $(TARGETS) $(HOME)/rpmbuild/BUILD
	./packaging/rpm/buildrpm.sh $(PKGNAME)
	cp $(HOME)/rpmbuild/RPMS/x86_64/$(PKGNAME)*.rpm .
