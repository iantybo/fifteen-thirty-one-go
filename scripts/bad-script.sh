#!/bin/bash

# This script has multiple shellcheck issues

# SC2086: Double quote to prevent globbing and word splitting
file=$1
cat $file

# SC2046: Quote this to prevent word splitting
for file in $(ls *.txt); do
    echo $file
done

# SC2006: Use $(...) notation instead of legacy backticked `...`
result=`date`
echo $result

# SC2164: Use 'cd ... || exit' in case cd fails
cd /some/directory
echo "Changed directory"

# SC2115: Use "${var:?}" to ensure this never expands to /
rm -rf $PREFIX/

# SC2162: read without -r will mangle backslashes
read line
echo $line

# SC2034: Variable appears unused
unused_var="test"

# SC2155: Declare and assign separately to avoid masking return values
export result=$(false)

# SC2223: This default assignment may cause DoS due to globbing
: ${var:=*.txt}

# SC2068: Double quote array expansions
args=("one" "two" "three")
echo ${args[@]}

# SC2103: Use a ( subshell ) to avoid having to cd back
cd /tmp
pwd
cd -

# SC2035: Use ./*glob* or -- *glob* so names with dashes won't become options
rm *.log
