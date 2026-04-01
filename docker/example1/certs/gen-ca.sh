#!/bin/bash

script_path=$(realpath $0)
script_dir=$(dirname $script_path)

cd $script_dir

cfssl gencert -initca ca.json | cfssljson -bare ca
