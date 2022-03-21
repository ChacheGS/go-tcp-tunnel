#!/bin/bash

function isIPv4() {
    local  ip=$1
    local  stat=1

    if [[ $ip =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
        OIFS=$IFS
        IFS='.'
        ip=($ip)
        IFS=$OIFS
        [[ ${ip[0]} -le 255 && ${ip[1]} -le 255 \
            && ${ip[2]} -le 255 && ${ip[3]} -le 255 ]]
        stat=$?
    fi
    return $stat
}

function certCmd() {
    local addr=$1
    local outputDir=$2
    local expireDays=$3

    isIPv4 $addr
    if [[ $? -eq 0 ]]; then
        subjectAltName="subjectAltName = IP:$addr"
    else
        subjectAltName="subjectAltName = DNS:$addr"
    fi

    set -e
    mkdir -p $outputDir

    echo $subjectAltName > $outputDir/san.txt
    openssl genrsa -out $outputDir/tls.key
    openssl req -new -key tls/tls.key -out $outputDir/tls.csr -subj "/CN=$addr"
    openssl x509 -in $outputDir/tls.csr -out $outputDir/tls.crt -req -signkey $outputDir/tls.key -days $expireDays -extfile $outputDir/san.txt
    echo "Successfully generated certificate in $outputDir"
}

function secretCmd() {
    set -e
    local name=$1
    local dir=$2
    local contextOption=$3
    local dryrunOption=$4
    cmd="kubectl create secret tls $name --cert=$dir/tls.crt --key=$dir/tls.key $contextOption $dryrunOption -o yaml > $dir/secret.yaml"
    echo $cmd
    eval $cmd
    [[ $dryrunOption != "" ]] && echo "Successfully generated k8s secret" || echo "Successfully installed secret $name"
}

function usage_exit() {
local msg=$1
cat <<EOF
$msg
usage:
    * Generate certificate
    certutil.sh cert --addr localhost
    certutil.sh cert --addr localhost --expire-days 36500 --dir tls

    * Generate Kubernetes TLS secret
    certutil.sh secert
    certutil.sh secert --install --context myk8scluster --dir tls
EOF
exit 9
}

cmd=$1
shift

for OPT in "$@"
do
    case $OPT in
        --addr | --addr=*)
            if [[ ${OPT#*=} == $OPT ]];then
                addr=$2
                shift 2
            else
                addr=${OPT#*=}
                shift 1
            fi
            ;;
        --expire-days | --expire-days=*)
            if [[ ${OPT#*=} == $OPT ]];then
                expireDays=$2
                shift 2
            else
                expireDays=${OPT#*=}
                shift 1
            fi
            ;;
        --dir | --dir=*)
            if [[ ${OPT#*=} == $OPT ]];then
                dir=$2
                shift 2
            else
                dir=${OPT#*=}
                shift 1
            fi
            ;;
        --name | --name=*)
            if [[ ${OPT#*=} == $OPT ]];then
                name=$2
                shift 2
            else
                name=${OPT#*=}
                shift 1
            fi
            ;;
        --context | --context=*)
            if [[ ${OPT#*=} == $OPT ]];then
                context=$2
                shift 2
            else
                context=${OPT#*=}
                shift 1
            fi
            ;;
        --install)
            isInstall=1
            shift 1
            ;;
        -*)
            usage_exit "ERR invalid option $OPT"
            ;;
    esac
done

case $cmd in
    cert)
        [[ $addr == "" ]] && usage_exit "ERR: --addr is required"
        [[ $dir == "" ]] && dir=tls
        [[ $expireDays == "" ]] && expireDays=36500
        certCmd $addr $dir $expireDays
        ;;
    secret)
        [[ $name == "" ]] && name=tcptunnel-cert
        [[ $dir == "" ]] && dir=tls
        [[ $context != "" ]] && contextOption="--context $context"
        [[ $isInstall -ne 1 ]] && dryrunOption="--dry-run"
        secretCmd $name $dir $contextOption $dryrunOption
        ;;
    *)
        usage_exit "ERR invalid command $cmd"
        ;;
esac