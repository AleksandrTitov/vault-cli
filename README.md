# Vault cli tool

**vault-cli** is a command line tool for initialize a new [HashiCorp Vault](https://www.vaultproject.io/) cluster or
unsealing of existing.

## Conditions
Following conditions necessary for work of tool:
* Vault register at [HashiCorp Consul](https://www.consul.io/) as a service;
* Vault URL scheme write to [HashiCorp Consul](https://www.consul.io/) Key\Value storage;
* [Consul Token](https://www.consul.io/docs/commands/index.html) should be set as environment variable CONSUL_HTTP_TOKEN;

## Using

**Initialization**
```
./vault-cli bootstrap
```
vault-cli tool finds all the Vault nodes at Consul Catalog and initializes the first Vault node , unseal nodes and return
to output Vault unseal keys and root token:

```
./vault-cli bootstrap
Unseal Key 1: +CyiPcb73fxh58KIffsG1HY1iBlEmfsldxTy01KO0SkV
Unseal Key 2: lx0fK56DpqZl88MNF0Tem53WzylD+ohBCRHfK6cubYzu
Unseal Key 3: vLJGBP5AOzY0Nly8wMP1jlNhsuvGcrfDuJ5UZrUj03/5
Unseal Key 4: tzEdApQxQ4PbXtCYGo+hRebw8l6fVQoh7lRjAp3YAeXv
Unseal Key 5: 8ZJyqmlIbaNYfvHUo8IY+VSRmEjHnTk8oeNM99h+Vxge

Initial Root Token: 336ff5e5-f45b-dc4e-9bdb-675ed510b589

* Unseal node: http://127.0.0.1:8220

Use key 1: +CyiPcb73fxh58KIffsG1HY1iBlEmfsldxTy01KO0SkV
Use key 2: lx0fK56DpqZl88MNF0Tem53WzylD+ohBCRHfK6cubYzu
Use key 3: vLJGBP5AOzY0Nly8wMP1jlNhsuvGcrfDuJ5UZrUj03/5

* Node unsealed

* Unseal node: http://127.0.0.1:8230

Use key 1: +CyiPcb73fxh58KIffsG1HY1iBlEmfsldxTy01KO0SkV
Use key 2: lx0fK56DpqZl88MNF0Tem53WzylD+ohBCRHfK6cubYzu
Use key 3: vLJGBP5AOzY0Nly8wMP1jlNhsuvGcrfDuJ5UZrUj03/5

* Node unsealed

```

Also, if at config parameter 'save' at section 'init' set 'true', all the output will be
save to file 'vault-keys'.

**Unseal**

```
echo <Unseal Key1> <Unseal Key2> | ./vault-cli unseal
```

the num of unseal keys should be equal of **threshold** parameter of config file vault-cli.conf

vault-cli finds all the Vault nodes at Consul Catalog Service and unseals them using encryption keys.

```
echo +CyiPcb73fxh58KIffsG1HY1iBlEmfsldxTy01KO0SkV lx0fK56DpqZl88MNF0Tem53WzylD+ohBCRHfK6cubYzu vLJGBP5AOzY0Nly8wMP1jlNhsuvGcrfDuJ5UZrUj03/5 | ./vault-cli unseal   
* Unseal node: http://127.0.0.1:8220

Use key 1: +CyiPcb73fxh58KIffsG1HY1iBlEmfsldxTy01KO0SkV
Use key 2: lx0fK56DpqZl88MNF0Tem53WzylD+ohBCRHfK6cubYzu
Use key 3: vLJGBP5AOzY0Nly8wMP1jlNhsuvGcrfDuJ5UZrUj03/5

* Node successful unsealed

* Unseal node: http://127.0.0.1:8230

Use key 1: +CyiPcb73fxh58KIffsG1HY1iBlEmfsldxTy01KO0SkV
Use key 2: lx0fK56DpqZl88MNF0Tem53WzylD+ohBCRHfK6cubYzu
Use key 3: vLJGBP5AOzY0Nly8wMP1jlNhsuvGcrfDuJ5UZrUj03/5

* Node successful unsealed
```

**Help**

To get help message necessary run vault-cli without a parameters:

```
./vault-cli
Usage: vault-cli <command>

Common commands:
* bootstrap      Bootstrap Vault cluster
* unseal         Unseal vault cluster

```

## Config file

Config file **vault-cli.conf** necessary for configuration of vault-cli.

The config file contains following sections with parameters:

**vault**

* scheme - path to Vault scheme at Consul KV storage;
* name - name of Vault service at Consul;

**init**

* save - save unseal keys and root token to file, value is true of false;
* shares - specifies the number of shares to split the master key into;
* threshold - Specifies the number of shares required to reconstruct the master key. This must be less than or equal 'shares';

**consul**

* addr - Consul address in format `<addr>:<port>`. For default using value 127.0.0.1:8500;
* scheme - Consul scheme, for default it's 'http';

## External packages

* [gopkg.in/gcfg.v1](https://github.com/go-gcfg/gcfg/) - reads configuration files into Go structs;