#!/bin/bash
ETCD_VERSION=3.0.17
TOKEN=my-etcd-token
CLUSTER_STATE=new
NAME_1=etcd-node-1
NAME_2=etcd-node-2
NAME_3=etcd-node-3
HOST_1=$ETCD_HOST_IP_1
HOST_2=$ETCD_HOST_IP_2
HOST_3=$ETCD_HOST_IP_3
ETCD_IMAGE_PREFFIX=$ETCD_IMAGE_NAME_PREFFIX
CLUSTER=${NAME_1}=http://${HOST_1}:2380,${NAME_2}=http://${HOST_2}:2380,${NAME_3}=http://${HOST_3}:2380
# For node 1
THIS_NAME=$ETCD_SELF_NAME
THIS_IP=$ETCD_SELF_HOST_IP


echo "HOST_1 = " $HOST_1
echo "HOST_2 = " $HOST_2
echo "HOST_3 = " $HOST_3
echo "ETCD_IMAGE_PREFFIX = " $ETCD_IMAGE_PREFFIX
echo "THIS_NAME = " $THIS_NAME
echo "THIS_IP = " $THIS_IP

docker run --net=host --restart=always --name etcd -v /var/lib/etcd:/data.etcd --detach ${ETCD_IMAGE_PREFFIX}/etcd-amd64:${ETCD_VERSION} \
        /usr/local/bin/etcd \
    --data-dir=data.etcd --name ${THIS_NAME} \
        --initial-advertise-peer-urls http://${THIS_IP}:2380 --listen-peer-urls http://${THIS_IP}:2380 \
        --advertise-client-urls http://${THIS_IP}:2379 --listen-client-urls http://${THIS_IP}:2379 \
        --initial-cluster ${CLUSTER} \
        --heartbeat-interval 500 \
        --election-timeout 5000 \
        --initial-cluster-state ${CLUSTER_STATE} --initial-cluster-token ${TOKEN}


