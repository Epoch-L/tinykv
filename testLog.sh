#!/bin/bash

for ((i=1;i<=3;i++));
do
echo "ROUND $i";
make project2c > ./test/out-"$i".log;
done
