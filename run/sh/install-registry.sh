#!/bin/bash

# 两个参数为仓库地址(不含http头)和镜像包的绝对路径
IMAGES_PATH=$2

if [[ $# < 2 ]];then
  echo "请传入正确的参数"
else
  # load img
  for a in $IMAGES_PATH/*.tar
  do
    sudo docker load -i $a
  done
  wait

  # tag IMG and push
  for b in $(sudo docker images | grep gcr.io/google_containers | awk {'print $1":"$2'})
  do
    c=$(echo $b | cut -d\/ -f 3)
    sudo docker tag $b $1/$c
    sudo docker rmi $b
    sudo docker images | grep $1
    sudo docker push $1/$c
    if [ $? -eq 0 ];then
      echo "IMG $1/$c PUSH SUCCESS!"
    else
      echo "IMG $1/$c PUSH FAILED!"
      echo 'push docker images failed'
      exit
    fi
  done

  for d in $(sudo docker images | grep quay.io | awk {'print $1":"$2'})
  do
    e=$(echo $d | awk  -F / {'print $2"/"$3'})
    sudo docker tag $d $1/$e
    sudo docker rmi $d
    sudo docker images | grep $1
    sudo docker push $1/$e
    if [ $? -eq 0 ];then
      echo "IMG $1/$e PUSH SUCCESS!"
    else
      echo "IMG $1/$e PUSH FAILED!"
      echo 'push docker images failed'
      exit
    fi
  done

  # fix yaml file
  #for i in $(ls $IMAGES_PATH| grep yml| awk -F / {'print $2":"$3'})
  for i in $(ls $IMAGES_PATH | grep yml | awk -F . {'print $1'})
  do
    sudo cp $IMAGES_PATH/$i.yml $IMAGES_PATH/$i.yaml
  done
    sudo sed -i "s/gcr.io\/google_containers/$1/g" $IMAGES_PATH/calico.yaml
    sudo sed -i "s/quay.io/$1/g" $IMAGES_PATH/kube-flannel.yaml
    sudo sed -i "s/quay.io/$1/g" $IMAGES_PATH/calico.yaml
fi

