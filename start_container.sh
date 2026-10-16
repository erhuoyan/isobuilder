#!/bin/bash
# jdos 云翼资源区容器入口脚本（固定初始化 + 应用拉起）
#
# 平台约定：前 28 行为固定内容，原样复制，负责把行云部署公钥注入容器
# （否则控制台目录查看/启停/网络工具/进程查询全部报 Permission denied）。
# ▼▼▼ 固定初始化开始（来自 jdos 官方示例，勿改）▼▼▼

#mirror_server='172.25.134.109'
mirror_server='mirrors.jdfin.local'
docker_init_script='docker_initialize.sh'
uname -m |grep -q aarch64 && docker_init_script='docker_initialize_arm.sh'
grep -qi ubuntu /etc/issue && mkdir -p /run/sshd && docker_init_script='docker_initialize_ubuntu.sh'
docker_init_url="http://${mirror_server}/docker/${docker_init_script}"

while true;do
  if ping -c 1 $mirror_server > /dev/null 2>&1;then
    echo "network is normal"
    if [[ `curl -I -m 1 -o /dev/null -s -w %{http_code} $docker_init_url` == 200 ]];then
      echo 'http test success'
      break
    else
      echo 'http test error'
    fi
  else
    echo "network is unnormal"
    ip ad sh
    ip ro sh
  fi
  sleep 2
done

curl -sL  $docker_init_url | bash
[[ -d /export/Logs ]] || { mkdir -p /export/Logs && chown admin:admin /export/Logs; }

######以上内容都是初始化系统依赖脚本，原样复制即可#######
######以下内容是启动应用内容，根据实际情况填写######

# 数据目录（NFS 挂载点，见 JDOS.md「持久化存储」一节）与日志目录
# （/export 是 jdos 本地盘）
mkdir -p /data/builds /data/isos /data/work /data/artifacts /data/uploads \
         /export/log/isobuilder
# 只有可写目录需要给 admin（服务以 admin 身份运行）：
#   /data                   构建工作区 + SQLite（NFS 挂载点，常为 root 属主）
#   /export/log/isobuilder  服务 stdout 日志
# /opt/isobuilder 是只读执行路径（二进制/前端/模板），落地已 755，无需 chown。
chown -R admin:admin /data /export/log/isobuilder

# start_before/after 钩子由 /home/admin/start.sh 内部调用，此处不再重复执行。
# /export/App/bin/ 的真实启停脚本需在部署前手动备好（复制路径见 JDOS.md
# 「首次部署」）；缺失时容器启动失败（CrashLoopBackOff）即暴露该前提——
# 失败必须可见，不静默兜底。

# 平台约定：必须切 admin 用户启动应用
su -m admin -c "bash /home/admin/start.sh"
