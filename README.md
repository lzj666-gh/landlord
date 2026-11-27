# 斗地主

基于 Go、SQLite 和 Phaser 的斗地主游戏，包含本地 AI 对局与多人房间。

## 项目结构

```text
cmd/landlord/   服务启动入口
common/         游戏规则、牌型与共享模型
conf/           服务配置
controllers/    HTTP 页面、登录、注册与注销接口
db/             SQLite 数据库
router/         HTTP/WebSocket 路由
service/        房间、牌桌、发牌与机器人逻辑
static/         Phaser 客户端、图片、音频与前端规则
templates/      游戏网页模板
rule.json       服务端牌型规则数据
```

源码的唯一启动入口是 `cmd/landlord`；编译产生的二进制和运行日志不会纳入版本控制。

## 运行

需要已安装 Go 并启用 Go Modules。所有命令都从项目根目录执行，因为配置、数据库和网页资源使用项目根目录作为工作目录。

直接启动（使用 8080 端口，适合普通用户权限）：

```sh
go run ./cmd/landlord -addr :8080
```

或先编译再启动：

```sh
go build -o landlord-server ./cmd/landlord
./landlord-server -addr :8080
```

然后访问 http://localhost:8080。默认配置端口在 `conf/app.conf` 中。

## 金币与外观

- 注册赠送 10000 金币，每局按「底注 500 × 叫分(1-3) × 倍数」结算：地主按两份底注收付，炸弹/王炸会让倍数翻倍。
- 金币低于 1000 时自动补足到 1000（结算与登录检查时都会处理）。
- 外观商店在 `/shop`：4 套牌桌背景、4 套牌面，购买后点「使用」穿戴，下次进牌桌生效。
- 账号金币存在 `account.coin`；已购商品和当前穿戴分别在 `account_item`、`account_skin`。

## 测试

```sh
go test ./...
```

游戏客户端代码来自 https://github.com/mailgyc/doudizhu 。

---

License

This project is released under the terms of the MIT license. See [LICENSE](LICENSE) for more
information or see https://opensource.org/licenses/MIT.
   
   
---
