# Copyright 2021 The Vitess Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

.ONESHELL:
SHELL = /bin/bash

PY_VERSION = 3.7
VIRTUALENV_PATH = benchmark

ANSIBLE_PATH = ansible
CONFIG_PATH = config/config.yaml
SCRIPTS_PATH = scripts
REPORTS_PATH = report

RUN_COMMIT = HEAD
RUN_INVENTORY_FILE = koz-inventory-unsharded-test.yml

.PHONY: install virtual_env oltp tpcc molecule_converge_all

virtual_env:
	test -d $(VIRTUALENV_PATH) || virtualenv -p python$(PY_VERSION) $(VIRTUALENV_PATH)

install: requirements.txt virtual_env $(VIRTUALENV_PATH)
	source $(VIRTUALENV_PATH)/bin/activate && \
	pip install -r ./requirements.txt
	python setup.py install
	ansible-galaxy install cloudalchemy.prometheus
	ansible-galaxy install cloudalchemy.node_exporter

install_dev_cli: virtual_env $(VIRTUALENV_PATH)
	source $(VIRTUALENV_PATH)/bin/activate && \
	python setup.py develop --user

oltp: virtual_env $(VIRTUALENV_PATH)
	source $(VIRTUALENV_PATH)/bin/activate && \
	clibench -c $(RUN_COMMIT) -s makefile_oltp -oltp -invf $(RUN_INVENTORY_FILE) \
													--config-file $(CONFIG_PATH) \
													--tasks-ansible-dir $(ANSIBLE_PATH) \
													--tasks-scripts-dir $(SCRIPTS_PATH) \
													--tasks-reports-dir $(REPORTS_PATH)

tpcc: virtual_env $(VIRTUALENV_PATH)
	source $(VIRTUALENV_PATH)/bin/activate && \
	clibench -c $(RUN_COMMIT) -s makefile_tpcc -tpcc -invf $(RUN_INVENTORY_FILE) \
													--config-file $(CONFIG_PATH) \
													--tasks--dir $(ANSIBLE_PATH) \
													--tasks-scripts-dir $(SCRIPTS_PATH) \
													--tasks-reports-dir $(REPORTS_PATH)

molecule_converge_all: virtual_env $(VIRTUALENV_PATH)
	source $(VIRTUALENV_PATH)/bin/activate && \
	cd $(ANSIBLE_PATH)/roles/vitess_build && \
	molecule create --scenario-name all && \
	OBJC_DISABLE_INITIALIZE_FORK_SAFETY=YES molecule converge --scenario-name all

molecule_dryrun_all: virtual_env $(VIRTUALENV_PATH)
	source $(VIRTUALENV_PATH)/bin/activate && \
	cd $(ANSIBLE_PATH)/roles/vitess_build && \
	OBJC_DISABLE_INITIALIZE_FORK_SAFETY=YES molecule converge --scenario-name all -- --skip-tags="oltp,tpcc"

tools:
	echo Installing git hooks
	chmod +x bootstrap.sh
	./bootstrap.sh
