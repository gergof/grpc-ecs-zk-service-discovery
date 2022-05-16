format:
	go fmt ./...

test:
	go test ./...

release:
	git tag v${VERSION}
	git push --tags
	curl https://sum.golang.org/lookup/github.com/gergof/grpc-ecs-zk-service-discovery@v${VERSION}
