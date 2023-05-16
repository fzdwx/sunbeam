#!/bin/bash
# https://github.com/zix99/bash-minimist
# Usage:
#
# Minimist is modeled after a similar script by the same name written for nodejs/javascript
# Its goal is to provide a minimal command-line parser for input into scripts
# Unlike classical approaches, no help-doc or pre-defined variable sets are defined
#

function __handleInvalidKey() {
  __handleError "Invalid key: '$1', as part of '${2:-$1}'"
  exit 1
}


function __sanitize() {
  local CLEAN=$*
  CLEAN=${CLEAN//[^a-zA-Z0-9_]/_}
  printf "%s" "$CLEAN"
}

ARGV=()
while (( "$#" )); do
  case "$1" in
    --) # Stop parsing args (the rest is positional)
      shift
      break
    ;;
    --*=*) # --abc=123
      KEY=${1%=*}
      KEY=$(__sanitize "$KEY")
      KEY=${KEY:2}
      declare "flag_${KEY}=${1#*=}" 2>/dev/null || __handleInvalidKey "$KEY"
      shift
    ;;
    --*) # --abc OR --abc 123
      KEY=$(__sanitize "$1")
      shift
      if [[ ! -z $1 && ${1:0:1} != '-' ]]; then
        declare "flag_${KEY:2}=$1" 2>/dev/null || __handleInvalidKey "$KEY"
        shift
      else
        declare "flag_${KEY:2}=y" 2>/dev/null || __handleInvalidKey "$KEY"
      fi
    ;;
    -*) # Multi-flag single-char args; -abc -a -b -C
      KEY=$1
      KEY=${KEY^^}
      for (( i=1; i<${#KEY}; i++ )); do
        declare "flag_${KEY:$i:1}=y" 2>/dev/null || __handleInvalidKey "${KEY:$i:1}" "$KEY"
      done
      shift
    ;;
    *) # positional args
      ARGV+=("$1")
      shift
    ;;
  esac
done

if [ ${#ARGV[@]} -ne 0 ]; then
  set -- "${ARGV[@]}" "$@"
else
  set -- "$@"
fi


# Cleanup non-exported things (since this will be sourced)
unset ARGV
unset KEY
unset _handleInvalidKey
unset __sanitize