all:

binary:
	cd cmd/neverdown && $(MAKE)

docker:
	$(MAKE) binary
	mv cmd/neverdown/neverdown .docker
	cd .docker && sudo docker build -t neverdown/neverdown .
	rm .docker/neverdown