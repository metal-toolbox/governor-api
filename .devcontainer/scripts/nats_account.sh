#!/bin/bash

sudo chown -Rh vscode:vscode /workspace/.devcontainer/nsc

echo "Dumping NATS user creds file"
nsc ${1} generate creds -a GOVERNOR -n USER > /tmp/user.creds

echo "Dumping NATS sys creds file"
nsc ${1} generate creds -a SYS -n sys > /tmp/sys.creds
