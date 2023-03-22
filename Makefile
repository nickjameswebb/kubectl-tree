.DEFAULT_GOAL: install

.PHONY: install
install:
	go mod tidy
	go install cmd/kubectl-tree.go

.PHONY: smoke-test
smoke-test: install
	kubectl tree pods -n kube-system
