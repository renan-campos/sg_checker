IMG ?= quay.io/rcampos/net-checker:latest

all: bin/netScout bin/netChecker

bin/netScout: cmd/scout.go
	go build -o bin/netScout cmd/scout.go

bin/netChecker: cmd/netChecker.go
	go build -o bin/netChecker cmd/netChecker.go

image:
	docker build -t $(IMG) .

push: image
	docker push $(IMG)

deploy: push
	kubectl apply -f manifests/

undeploy:
	kubectl delete -f manifests/

clean:
	rm -f bin/netScout bin/netChecker
