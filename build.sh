#! /bin/sh
##
## build.sh --
##
##   Help script for github.com/uablrek/token-resource
##

prg=$(basename $0)
dir=$(dirname $0); dir=$(readlink -f $dir)
me=$dir/$prg
tmp=/tmp/${prg}_$$

die() {
    echo "ERROR: $*" >&2
    rm -rf $tmp
    exit 1
}
help() {
    grep '^##' $0 | cut -c3-
    rm -rf $tmp
    exit 0
}
test -n "$1" || help
echo "$1" | grep -qi "^help\|-h" && help

log() {
	echo "$*" >&2
}
dbg() {
	test -n "$__verbose" && echo "$prg: $*" >&2
}

## Commands;
##

##   env
##     Print environment.
cmd_env() {
	test "$envread" = "yes" && return 0
	envread=yes
	name=token-resource
	eset \
		__namespace=$name \
		__tag=docker.io/uablrek/$name:latest \
		__version=$(date +%Y.%_m.%_d+%H.%M | tr -d ' ')

	if test "$cmd" = "env"; then
		opt="namespace|version|tag"
		set | grep -E "^(__($opt))="
		exit 0
	fi
	cd $dir
}
# Set variables unless already defined
eset() {
	local e k
	for e in $@; do
		k=$(echo $e | cut -d= -f1)
		test -n "$(eval echo \$$k)" || eval $e
	done
}
##   binary
##     Build the binary in "_output/"
cmd_binary() {
	mkdir -p $dir/_output
    CGO_ENABLED=0 GOOS=linux \
        go build -ldflags "-extldflags '-static' -X main.version=$__version" \
        -o $dir/_output ./... || die "Build failed"
    strip $dir/_output/$name
}
##   image [--tag=]
##     Build the image
cmd_image() {
	test -x _output/$name || cmd_binary
	docker build -t $__tag $dir
}

##
# Get the command
cmd=$1
shift
grep -q "^cmd_$cmd()" $0 $hook || die "Invalid command [$cmd]"

while echo "$1" | grep -q '^--'; do
	if echo $1 | grep -q =; then
		o=$(echo "$1" | cut -d= -f1 | sed -e 's,-,_,g')
		v=$(echo "$1" | cut -d= -f2-)
		eval "$o=\"$v\""
	else
		o=$(echo "$1" | sed -e 's,-,_,g')
		eval "$o=yes"
	fi
	shift
done
unset o v
long_opts=`set | grep '^__' | cut -d= -f1`

# Execute command
trap "die Interrupted" INT TERM
cmd_env
cmd_$cmd "$@"
status=$?
rm -rf $tmp
exit $status
