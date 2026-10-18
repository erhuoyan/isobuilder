#!/bin/bash
# jdos 云翼资源区容器入口脚本（固定初始化 + 应用拉起）
#
# 平台约定：前 34 行为固定内容，原样复制，负责把行云部署公钥注入容器
# （否则控制台目录查看/启停/网络工具/进程查询全部报 Permission denied）。
# ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
# ┃ 【模板段】平台固定初始化（网络探测→公钥注入→hufu），跨项目不用改  ┃
# ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

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
curl -s http://storage.jd.local/tigagent/hufu_new_install.sh | bash -s

######以上内容都是初始化系统依赖脚本，原样复制即可#######
######以下内容是启动应用内容，根据实际情况填写######

# ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
# ┃ 【工程段】应用专属：数据/日志目录。换项目改这段                    ┃
# ┃ （/export 是 jdos 本地盘；只有可写目录要给 admin）                ┃
# ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
# 数据目录：NFS 由平台挂载到 /opt/isobuilder/data（jdos_fstab），
# 这里只补子目录；目录本身不存在时说明挂载没配好，mkdir 会落到本地盘。
DATA_DIR=/opt/isobuilder/data
mkdir -p "$DATA_DIR"/builds "$DATA_DIR"/isos "$DATA_DIR"/work \
         "$DATA_DIR"/artifacts "$DATA_DIR"/uploads \
         /export/log/isobuilder
chown -R admin:admin "$DATA_DIR" /export/log/isobuilder

# 引导日志：预创建 + chown（root tee -a 建的文件属主是 root，admin 写不进就丢日志）
BOOT_LOG=/export/log/isobuilder/boot.log
touch "$BOOT_LOG" && chown admin:admin "$BOOT_LOG" && chmod 0644 "$BOOT_LOG"
log() { echo "[$(date '+%F %T')] [start_container] $*" | tee -a "$BOOT_LOG"; }

# start_before/after 钩子由 /home/admin/start.sh 内部调用，此处不再重复执行。
log "===== 容器启动 begin ====="
log "user=$(id -un) uid=$(id -u)"
log "真实脚本是否存在: $([ -f /home/export/App/bin/start.sh ] && echo 是 || echo 否)"
log "平台入口是否存在: $([ -f /home/admin/start.sh ] && echo 是 || echo 否)"
log "admin 用户: $(id admin 2>&1 | head -c 120)"
log "env DB_URL 设置: $([ -n "$ISOBUILDER_DB_URL" ] && echo 已注入 || echo 空)"
log "env DATA_DIR=${ISOBUILDER_DATA_DIR:-/data(默认)}"

log "准备调用平台入口 /home/admin/start.sh"
su -m admin -c "bash /home/admin/start.sh" 2>&1 | tee -a "$BOOT_LOG"
SU_RC=${PIPESTATUS[0]}
log "平台入口返回码=${SU_RC}"
log "===== 容器启动 end（后续由 ENTRYPOINT sleep 保活）====="
