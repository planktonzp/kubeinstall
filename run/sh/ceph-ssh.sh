#!/bin/bash
      if [ ! -e ~/.ssh/id_rsa.pub ]
        then
          echo "公/私密钥对未找到，初始化中..."
          sudo expect -c "
              spawn sudo ssh-keygen
              expect {
                  \"ssh/id_rsa):\" {send \"\r\";exp_continue}
                  \"Over\" {send \"n\r\";exp_continue}
                  \"passphrase):\" {send \"\r\";exp_continue}
                  \"again:\" {send \"\r\";exp_continue}
              }
          " >/dev/null 2>&1
          if [ -e ~/.ssh/id_rsa.pub ]
          then
              echo "成功创建公/私密钥对"
          else
              echo "公/私密钥对创建失败"
              exit 1
          fi
        fi
      for i in $@
      do
          USER=$(echo $i|cut -d@ -f1)
	  echo $USER
          IP=$(echo $i|cut -d@ -f2)
	  echo $IP
          PASS=$(echo $i|cut -d@ -f3)
	  echo $PASS
          INFO=$USER@$IP
	  echo $INFO
          sudo expect -c "
              spawn sudo ssh-copy-id -i /root/.ssh/id_rsa.pub $INFO
              expect {
                  \"(yes/no)?\" {send \"yes\r\";exp_continue}
                  \"password:\" {send \"$PASS\r\";exp_continue}
              }
          "  #各种日志会记录在这个文件内

#          echo "原配置已备份为config.bak"
#          cp ~/.ssh/config ~/.ssh/config.bak
#            echo "Host $IP" >> ~/.ssh/config
#            echo "   Hostname $IP" >> ~/.ssh/config
#            echo "   User $USER" >> ~/.ssh/config
#            echo " 文件已更新为： "
#            cat $HOME/.ssh/config
      done
#

