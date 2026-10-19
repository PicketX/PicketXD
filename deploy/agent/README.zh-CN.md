# 携带源码的独立 Agent 安装包

> [English (default)](README.md) | 简体中文

Agent使用GPL-3.0-or-later，Apache Controller另行分发。完整保留安装包、LICENSE、
上游许可、源码及SHA清单；这是工程验收，不是自动法律合规认证或OEM商业授权。

## 构建、校验与搬迁

干净且已提交的源码需要Go1.27.x、Node、Bash、Git、GNU tar/coreutils、file/readelf。
运行`bash scripts/build-agent-bundle.sh /absolute/new/bundle`，目标必须不存在，
悬空符号链接也拒绝。不覆盖旧目录、不安装宿主；失败保留部分输出供排查。先通过可信
发布渠道确认源码身份，再核对全部SHA256SUMS后运行二进制；摘要自身不能认证发布者。
整包搬迁/解压并保留执行位，Linux-amd64程序运行时不需要Go/npm或共享库。

## 源码与离线重建

source/PicketXD包含准确提交的Agent源码、构建脚本及官方go mod vendor收集的
构建/测试依赖。按该命令约定不包含上游依赖自身的测试。licenses保存实际上游许可/
NOTICE以及Go运行时LICENSE/PATENTS，并记录版本索引；Go工具链另向其发布者获取。

安装Go1.27.x后进入source/PicketXD，设置GOPROXY=off、GOSUMDB=off、GOTOOLCHAIN=local、
CGO_ENABLED=0、GOOS=linux、GOARCH=amd64，然后执行
`go build -mod=vendor -trimpath -o /absolute/new/picketxd ./cmd/picketxd`。
适当Linux环境可运行`go test -mod=vendor ./...`；特权场景须显式开启并使用可丢弃
网络命名空间。使用包内vendor编译不依赖网络/模块缓存。不宣称逐字节可重现：源码导出
没有Git元数据，SOURCE.json记录原提交、源码树、平台和Go版本。

## 首次使用与限制

无关工作目录/空PATH下可运行bin/picketxd --help；prepare --once只验证/持久化策略，
从不证明内核执行。参阅包内systemd说明及源码deploy/controller/README.zh-CN.md，
配置独立Token文件、权威provider、独占mark、双栈目标与持久状态。不盲目安装样例单元/
策略/mark，私有状态不要存入公开安装包。本包不会自动启用宿主服务。

原生验收实际打包/解压、空模块缓存离线重建及空PATH运行；既有真实Linux浏览器→
Controller→Agent→可信Nginx→Gitea链路直接执行包内二进制，保留重启、双栈流量、拒绝、
独立Grant及严格撤销。宿主开机、Controller服务安装及真实备份恢复仍另行验收。
