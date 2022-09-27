.DEFAULT_GOAL: build

.PHONY: install
install:
	go mod tidy
	go install cmd/kubectl-tree.go
