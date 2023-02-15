#!/bin/bash

for ((i=1;i<=1;i++));
do
echo "ROUND $i";
make project2c > ./test/out-"$i".log;
done
