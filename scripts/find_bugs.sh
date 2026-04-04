#!/bin/bash
echo "--- Searching for unlocked map accesses in Go ---"
grep -n "\[.*\] =" backend/*.go | grep -v "mu.Lock"

echo "--- Searching for goroutine leaks / unhandled panics ---"
grep -n "go func" backend/*.go

echo "--- Searching for defer in loops ---"
grep -A 5 "for " backend/*.go | grep "defer "

echo "--- Searching for missing error checks ---"
grep -n "[a-zA-Z0-9_]* \:= " backend/*.go | grep -v "err" | grep -E "os\.Open|ioutil\.ReadAll|json\.Unmarshal|http\.Get"

echo "--- Searching for reactive state mutations in Vue ---"
grep -rn "\.value =" frontend/src/components/ | grep -E "props\.|inject\("

