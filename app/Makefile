# Delegate all targets to the root Makefile
ROOT := $(shell cd .. && pwd)

%:
	@$(MAKE) -C $(ROOT) $@
