
PG.Game = function(game) {

    this.roomId = 1;
    this.players = [];

    this.titleBar = null;
    this.tableId = 0;
    this.shotLayer = null;

    this.tablePoker = [];
    this.tablePokerPic = {};

    this.lastShotPlayer = null;

    this.whoseTurn = 0;

    this.seatCount = 0;        //牌桌上现在有几个人
    this.dealt = false;        //是不是正在打牌
    this.addRobotButton = null;
    this.gameOverTimer = null;
    this.tableLayer = null;    //选择牌桌的浮层
    this.callLayer = null;     //叫分按钮，回合过了/开新局都要收掉
    this.gameOverPanel = null; //一局结束后的结算面板
    this.restartButton = null;
    this.restartPending = false;
    this.landlordSeat = -1;    //地主座位
    this.pendingBottom = false;//三张底牌还在飞向地主
    this.bottomTimer = null;
    this.coinText = null;      //右下角金币
    this.coinIcon = null;      //金币图标
    this.tableBg = null;       //账号当前穿戴的背景

};

PG.Game.prototype = {

    init: function(roomId) {
        this.roomId = roomId;
    },

    debug_log(obj) {
    console.log('*******');
    console.log(obj);
    console.log('********');
    },

	create: function () {
        this.stage.backgroundColor = '#182d3b';
        this.createTableBackground();

        this.players.push(PG.createPlay(0, this));
        this.players.push(PG.createPlay(1, this));
        this.players.push(PG.createPlay(2, this));
        this.players[0].updateInfo(PG.playerInfo.uid, PG.playerInfo.username);
        //另外两个座位先空着，人没进来之前可以点空位加机器人
        this.players[1].updateInfo(-1, '');
        this.players[2].updateInfo(-1, '');
        PG.Socket.connect(this.onopen.bind(this), this.onmessage.bind(this), this.onerror.bind(this));

        this.createTitleBar();
        this.titleBar.text = '房间 ' + this.roomId + ' · 选择牌桌';
        this.createCoinHud();
        this.createAddRobotButton();
        this.enableEmptySeatClick();
	},
	
	onopen: function() {
	    console.log('socket onopen');
        PG.Socket.send([PG.Protocol.REQ_JOIN_ROOM, this.roomId]);
	},

    onerror: function() {
        console.log('socket connect onerror');
    },

	send_message: function(request) {
        PG.Socket.send(request);
	},
	
	onmessage: function(packet) {
	    var opcode = packet[0];
	    switch(opcode) {
            case PG.Protocol.RSP_JOIN_ROOM:
                //房间号由 /room/{rid} 决定，进房间后先选牌桌
                this.createTableLayer(packet[1]);
                break;
            case PG.Protocol.RSP_TABLE_LIST:
                if (this.tableId) {
                    //已经坐在牌桌里了，列表只是顺手回的，不能盖在牌局上
                    break;
                }
                this.createTableLayer(packet[1]);
                break;
            case PG.Protocol.RSP_NEW_TABLE:
                this.tableId = packet[1];
                this.titleBar.text = '牌桌 ' + this.tableId;
                break;
	        case PG.Protocol.RSP_JOIN_TABLE:
                this.tableId = packet[1];
                this.titleBar.text = '牌桌 ' + this.tableId;
                this.hideTableLayer();
                this.updateSeats(packet[2]);
                break;
            case PG.Protocol.RSP_DEAL_POKER:
                var playerId = packet[1];
                var pokers = packet[2];
                console.log(pokers);
                this.dealt = true;
                this.clearGameOverTimer();
                this.hideGameOverPanel();
                this.updateAddRobotButton();
                this.dealPoker(pokers, packet[3]);
                this.whoseTurn = this.uidToSeat(playerId);
                if (packet[3] == PG.GameState.PLAYING) {
                    //断线重连：这一局已经在出牌，恢复桌面上的牌，不要再走叫分流程
                    var lastShotSeat = packet[5] ? this.uidToSeat(packet[5]) : -1;
                    var lordSeat = packet[6] ? this.uidToSeat(packet[6]) : -1;
                    this.tablePoker = packet[4] || [];
                    this.lastShotPlayer = lastShotSeat >= 0 ? this.players[lastShotSeat] : this.players[this.whoseTurn];
                    if (lordSeat >= 0) {
                        this.players[lordSeat].setLandlord();
                    }
                    if (this.whoseTurn == 0) {
                        this.startPlay();
                    }
                } else {
                    this.startCallScore(0);
                }
                break;
            case PG.Protocol.RSP_CALL_SCORE:
                var playerId = packet[1];
                var score = packet[2];
                var callend = packet[3];
                this.debug_log(callend);
                this.whoseTurn = this.uidToSeat(playerId);
                //this.debug_log(playerId);

                var hanzi = ['不叫', "一分", "两分", "三分"];
                this.players[this.whoseTurn].say(hanzi[score]);
                if (!callend) {
                    this.whoseTurn = (this.whoseTurn + 1) % 3;
                    this.startCallScore(score);
                } else {
                    this.hideCallScore();
                }
                break;
            case PG.Protocol.RSP_SHOW_POKER:
                this.whoseTurn = this.uidToSeat(packet[1]);
                this.landlordSeat = this.whoseTurn;
                this.tablePoker[0] = packet[2][0];
                this.tablePoker[1] = packet[2][1];
                this.tablePoker[2] = packet[2][2];
                this.players[this.whoseTurn].setLandlord();
                this.showLastThreePoker();
                break;
            case PG.Protocol.RSP_SHOT_POKER:
                this.handleShotPoker(packet);
                break;
            case PG.Protocol.RSP_GAME_OVER:
                var winner = packet[1];
                // [2] 本局金币增减（已按地主/农民阵营算好），[5] 结算后余额，
                // [6] 本局单家底注，[7] 倍数，[8] 补足到 1000 的金额
                var settlement = {
                    delta: Number(packet[2] || 0),
                    coin: Number(packet[5] || 0),
                    base: Number(packet[6] || 0),
                    multiple: Number(packet[7] || 1),
                    refill: Number(packet[8] || 0)
                };
                this.lastSettlement = settlement;
                this.updateCoin(settlement.coin);

                this.dealt = false;
                this.updateAddRobotButton();

                var loserASeat = this.uidToSeat(packet[3][0]);
                this.players[loserASeat].replacePoker(packet[3], 1);
                this.players[loserASeat].reDealPoker();

                var loserBSeat = this.uidToSeat(packet[4][0]);
                this.players[loserBSeat].replacePoker(packet[4], 1);
                this.players[loserBSeat].reDealPoker();
//                 this.players[loserBSeat].removeAllPoker();
//               this.players[loserASeat].pokerInHand = [];

                this.whoseTurn = this.uidToSeat(winner);

                //等最后的牌摆好再弹出结算，让玩家自己选“再来一把”还是“返回房间”
                this.clearGameOverTimer();
                this.gameOverTimer = this.game.time.events.add(1500, function () {
                    this.gameOverTimer = null;
                    this.showGameOverPanel(winner, settlement);
                }, this);
                break;
            case PG.Protocol.RSP_CHEAT:
                var seat = this.uidToSeat(packet[1]);
                this.players[seat].replacePoker(packet[2], 0);
                this.players[seat].reDealPoker();
                break;
            case PG.Protocol.RSP_RESTART:
                if (packet[1] === false) {
                    //人不够，服务端没开新一局
                    this.restartPending = false;
                    if (this.restartButton) {
                        this.restartButton.text = '再来一把';
                        this.restartButton.inputEnabled = true;
                    }
                    this.showToast(packet[2]);
                    break;
                }
                this.dealt = false;
                this.clearGameOverTimer();
                this.hideGameOverPanel();
                this.updateAddRobotButton();
                this.restart();
                break;
            case PG.Protocol.RSP_LEAVE_TABLE:
                if (packet[1] === false) {
                    this.showToast(packet[2]);
                    break;
                }
                this.leaveTableUI();
                break;
            case PG.Protocol.RSP_ADD_ROBOT:
                if (!packet[1]) {
                    this.showToast(packet[2]);
                }
                break;
            default:
                console.log("UNKNOWN PACKET:", packet)
	    }
	},

    //收起“不出/提示/出牌”按钮（开新局、离开牌桌都要收）
    clearShotButtons: function () {
        if (this.players[0] && this.players[0].shotLayer) {
            this.players[0].shotLayer.forEach(function (child) {
                child.kill();
            });
        }
    },

    cleanWorld: function () {
        this.clearBottomPoker();
        this.hideCallScore();
        this.clearShotButtons();
        for (var i = 0; i < this.players.length; i++) {
            this.players[i].cleanPokers();
            if (this.players[i].uiLeftPoker) {
                this.players[i].uiLeftPoker.kill();
            }
            this.players[i].isLandlord = false;
            //空位还是要显示默认头像
            this.players[i].uiHead.loadTexture(this.players[i].uid == -1 ? 'avatar_default' : 'avatar_farmer');
        }

        this.clearTablePokers();
    },

    //桌面中间的牌（打出去的牌 + 还没飞完的底牌）全部销毁，不留到下一局
    clearTablePokers: function () {
        for (var pokerId in this.tablePokerPic) {
            if (this.tablePokerPic.hasOwnProperty(pokerId) && this.tablePokerPic[pokerId]) {
                this.tablePokerPic[pokerId].destroy();
            }
        }
        for (var i = 0; i < this.tablePoker.length; i++) {
            var item = this.tablePoker[i];
            if (item && item.destroy) {
                item.destroy();
            }
        }
        this.tablePoker = [];
        this.tablePokerPic = {};
    },

    restart: function () {
        this.clearBottomPoker();
        this.hideCallScore();
        this.clearShotButtons();
        this.clearTablePokers();
        this.lastShotPlayer = null;
        this.whoseTurn = 0;
        for (var i = 0; i < 3; i++) {
            var player = this.players[i];
            player.cleanPokers();
            player.isLandlord = false;
            player.hintPoker = [];
            //空位还是要显示默认头像
            player.uiHead.loadTexture(player.uid == -1 ? 'avatar_default' : 'avatar_farmer');
            if (player.uiLeftPoker) {
                player.uiLeftPoker.kill();
            }
        }
    },

	update: function () {
	},

	uidToSeat: function (uid) {
	    for (var i = 0; i < 3; i++) {
//	        this.debug_log(this.players[i].uid);
	        if (uid == this.players[i].uid)
	            return i;
	    }
	    console.log('ERROR uidToSeat:' + uid);
	    return -1;
	},

    //牌桌上的人按座位顺序排好，人没到齐的空位显示“等待玩家加入”
    updateSeats: function (playerIds) {
        playerIds = playerIds || [];
        var total = playerIds.length;
        this.seatCount = total;
        var myIndex = -1;
        for (var i = 0; i < total; i++) {
            if (playerIds[i][0] == this.players[0].uid) {
                myIndex = i;
                break;
            }
        }
        if (myIndex < 0) {
            //还没找到自己，先按空位显示
            for (var empty = 1; empty < 3; empty++) {
                this.players[empty].updateInfo(-1, '');
            }
            this.updateAddRobotButton();
            return;
        }
        this.players[0].updateInfo(playerIds[myIndex][0], playerIds[myIndex][1]);
        for (var seat = 1; seat < 3; seat++) {
            if (seat >= total) {
                //人还没到齐，空位留着给后面来的人或者机器人
                this.players[seat].updateInfo(-1, '');
                continue;
            }
            var index = (myIndex + seat) % total;
            this.players[seat].updateInfo(playerIds[index][0], playerIds[index][1]);
        }
        this.updateAddRobotButton();
    },

    //人数没满并且没在打牌的时候才能加机器人
    createAddRobotButton: function () {
        var btn = this.game.add.button(this.game.world.centerX, 26, 'ui_addrobot', this.onAddRobotDown, this);
        btn.anchor.set(0.5, 0);
        this.addRobotButton = btn;
        this.updateAddRobotButton();
    },

    updateAddRobotButton: function () {
        if (!this.addRobotButton) {
            return;
        }
        var show = this.seatCount > 0 && this.seatCount < 3 && !this.dealt;
        this.addRobotButton.visible = show;
        if (show) {
            this.game.world.bringToTop(this.addRobotButton);
        }
    },

    onAddRobotDown: function () {
        this.requestAddRobot();
    },

    //空座位可以直接点，点了就加一个机器人
    enableEmptySeatClick: function () {
        for (var seat = 1; seat < 3; seat++) {
            var head = this.players[seat].uiHead;
            head.seatIndex = seat;
            head.inputEnabled = true;
            head.input.useHandCursor = true;
            head.events.onInputDown.add(this.onEmptySeatDown, this);
        }
    },

    onEmptySeatDown: function (head) {
        if (!this.tableId) {
            //还没坐下，空位只是背景
            return;
        }
        if (this.players[head.seatIndex].uid != -1) {
            //座位上已经有人了
            return;
        }
        this.requestAddRobot();
    },

    requestAddRobot: function () {
        if (!this.tableId) {
            return;
        }
        if (this.seatCount >= 3 || this.dealt) {
            this.showToast('现在不能添加机器人');
            return;
        }
        this.send_message([PG.Protocol.REQ_ADD_ROBOT]);
    },

    showToast: function (msg) {
        if (!msg) {
            return;
        }
        var style = {font: "22px Arial", fill: "#ffd479", align: "center", stroke: "#182d3b", strokeThickness: 4};
        var text = this.game.add.text(this.game.world.centerX, 56, msg, style);
        text.anchor.set(0.5, 0);
        this.game.world.bringToTop(text);
        this.game.time.events.add(2200, text.destroy, text);
    },

    //新的一局开始了就别再弹上一局的结算
    clearGameOverTimer: function () {
        if (this.gameOverTimer) {
            //Phaser 2 的 TimerEvent 没有 remove()，要从 Timer 里摘掉
            this.game.time.events.remove(this.gameOverTimer);
            this.gameOverTimer = null;
        }
    },

    //按账号穿戴铺一层背景；没有背景图时退回纯色桌面
    createTableBackground: function () {
        if (!this.game.cache.checkImageKey('tablebg')) {
            return;
        }
        var bg = this.game.add.image(0, 0, 'tablebg');
        bg.scale.set(this.game.world.width / bg.width, this.game.world.height / bg.height);
        bg.sendToBack();
        this.tableBg = bg;
    },

    //右下角金币，结算后立刻更新
    createCoinHud: function () {
        //放在右下角：右上角被对手头像和名字占着，顶部中间是“加机器人”按钮
        var style = {font: "18px Arial", fill: "#ffd479", align: "right", stroke: "#0d1a22", strokeThickness: 3};
        this.coinText = this.game.add.text(this.game.world.width - 12, this.game.world.height - 8, '', style);
        this.coinText.anchor.set(1, 1);
        this.coinIcon = this.game.add.image(0, this.game.world.height - 7, 'icon_coin');
        this.coinIcon.anchor.set(1, 1);
        this.coinIcon.scale.set(22 / this.coinIcon.width);
        this.game.world.bringToTop(this.coinIcon);
        this.game.world.bringToTop(this.coinText);
        this.updateCoin((PG.playerInfo && PG.playerInfo.coin) || 0);
    },

    updateCoin: function (coin) {
        if (typeof coin !== 'number') {
            return;
        }
        if (PG.playerInfo) {
            PG.playerInfo.coin = coin;
        }
        if (this.coinText) {
            this.coinText.text = '金币 ' + coin;
            this.coinIcon.x = this.game.world.width - 12 - this.coinText.width - 6;
            this.game.world.bringToTop(this.coinIcon);
            this.game.world.bringToTop(this.coinText);
        }
    },

    //一局结束后的结算面板：再来一把 / 返回房间
    showGameOverPanel: function (winnerUid, settlement) {
        if (this.gameOverPanel) {
            return;
        }
        var width = this.game.world.width;
        var height = this.game.world.height;
        var group = this.game.add.group();
        this.game.world.bringToTop(group);

        var dim = this.game.make.graphics(0, 0);
        dim.beginFill(0x000000, 0.6);
        dim.drawRect(0, 0, width, height);
        dim.endFill();
        dim.inputEnabled = true; //挡住下面的牌，免得误点
        group.add(dim);

        var panelW = 460;
        var panelH = 330;
        var px = width / 2 - panelW / 2;
        var py = height / 2 - panelH / 2;
        var panel = this.game.make.graphics(0, 0);
        panel.beginFill(0x1f3b4d, 1);
        panel.lineStyle(2, 0x4d8fb0, 1);
        panel.drawRoundedRect(px, py, panelW, panelH, 14);
        panel.endFill();
        group.add(panel);

        var winnerSeat = this.uidToSeat(winnerUid);
        var landlordWin = winnerSeat >= 0 ? this.players[winnerSeat].isLandlord : false;
        var title = this.game.make.text(width / 2, py + 50, landlordWin ? '地主赢' : '农民赢',
            {font: '34px Arial', fill: '#ffd479', align: 'center', stroke: '#0d1a22', strokeThickness: 4});
        title.anchor.set(0.5, 0.5);
        group.add(title);

        var trophy = this.game.make.image(width / 2 - title.width / 2 - 22, py + 50, 'icon_trophy');
        trophy.anchor.set(1, 0.5);
        trophy.scale.set(34 / trophy.width);
        group.add(trophy);

        var mine = winnerUid == this.players[0].uid;
        var sub = this.game.make.text(width / 2, py + 96, mine ? '你赢了！' : '你输了',
            {font: '22px Arial', fill: '#dce8ee', align: 'center'});
        sub.anchor.set(0.5, 0.5);
        group.add(sub);

        var delta = settlement ? settlement.delta : 0;
        var deltaColor = delta > 0 ? '#7ee2a8' : (delta < 0 ? '#ff9a9a' : '#dce8ee');
        var deltaText = (delta > 0 ? '+' : '') + delta + ' 金币';
        var deltaLabel = this.game.make.text(width / 2, py + 140, deltaText,
            {font: '30px Arial', fill: deltaColor, align: 'center', stroke: '#0d1a22', strokeThickness: 3});
        deltaLabel.anchor.set(0.5, 0.5);
        group.add(deltaLabel);

        var deltaCoin = this.game.make.image(width / 2 - deltaLabel.width / 2 - 18, py + 140, 'icon_coin');
        deltaCoin.anchor.set(1, 0.5);
        deltaCoin.scale.set(24 / deltaCoin.width);
        group.add(deltaCoin);

        var balance = settlement ? settlement.coin : ((PG.playerInfo && PG.playerInfo.coin) || 0);
        var balanceText = this.game.make.text(width / 2, py + 180, '余额 ' + balance + ' 金币',
            {font: '19px Arial', fill: '#c9b26a', align: 'center'});
        balanceText.anchor.set(0.5, 0.5);
        group.add(balanceText);

        var detail = '单家基数 ' + (settlement ? settlement.base : 0) +
            ' · 倍数 ×' + (settlement ? settlement.multiple : 1);
        var detailText = this.game.make.text(width / 2, py + 212, detail,
            {font: '15px Arial', fill: '#9fb4c2', align: 'center'});
        detailText.anchor.set(0.5, 0.5);
        group.add(detailText);

        if (settlement && settlement.refill > 0) {
            var refillText = this.game.make.text(width / 2, py + 240, '金币低于 ' + 1000 + '，已补足 ' + settlement.refill,
                {font: '15px Arial', fill: '#ffd479', align: 'center'});
            refillText.anchor.set(0.5, 0.5);
            group.add(refillText);
        }

        this.createPanelButton(group, width / 2 - 110, py + 288, '再来一把', 0x31a66a, this.onRestartDown);
        this.createPanelButton(group, width / 2 + 110, py + 288, '返回房间', 0x2b536b, this.onLeaveTableDown);
        this.gameOverPanel = group;
        this.restartPending = false;
    },

    createPanelButton: function (group, cx, cy, label, color, handler) {
        var bw = 180;
        var bh = 48;
        var bg = this.game.make.graphics(0, 0);
        bg.beginFill(color, 1);
        bg.drawRoundedRect(cx - bw / 2, cy - bh / 2, bw, bh, 10);
        bg.endFill();
        group.add(bg);

        var text = this.game.make.text(cx, cy, label, {font: '22px Arial', fill: '#ffffff', align: 'center'});
        text.anchor.set(0.5, 0.5);
        text.inputEnabled = true;
        text.input.useHandCursor = true;
        text.events.onInputDown.add(handler, this);
        group.add(text);
        if (label == '再来一把') {
            this.restartButton = text;
        }
        return text;
    },

    onRestartDown: function () {
        if (this.restartPending) {
            return;
        }
        this.restartPending = true;
        if (this.restartButton) {
            this.restartButton.text = '等待开局…';
            this.restartButton.inputEnabled = false;
        }
        this.send_message([PG.Protocol.REQ_RESTART]);
    },

    onLeaveTableDown: function () {
        this.send_message([PG.Protocol.REQ_LEAVE_TABLE]);
    },

    hideGameOverPanel: function () {
        if (this.gameOverPanel) {
            this.gameOverPanel.destroy();
            this.gameOverPanel = null;
        }
        this.restartButton = null;
        this.restartPending = false;
    },

    //离开牌桌后把游戏画面恢复成“选牌桌”的状态
    leaveTableUI: function () {
        this.clearGameOverTimer();
        this.hideGameOverPanel();
        this.tableId = 0;
        this.seatCount = 0;
        this.dealt = false;
        this.whoseTurn = 0;
        this.lastShotPlayer = null;
        this.cleanWorld();
        this.titleBar.text = '房间 ' + this.roomId + ' · 选择牌桌';
        this.players[0].updateInfo(PG.playerInfo.uid, PG.playerInfo.username);
        for (var seat = 1; seat < 3; seat++) {
            this.players[seat].updateInfo(-1, '');
        }
        this.updateAddRobotButton();
    },

    //新一局/离开牌桌时把还没发完的底牌状态清掉
    clearBottomPoker: function () {
        if (this.bottomTimer) {
            this.game.time.events.remove(this.bottomTimer);
            this.bottomTimer = null;
        }
        this.pendingBottom = false;
        this.landlordSeat = -1;
    },

    hideTableLayer: function () {
        if (this.tableLayer) {
            this.tableLayer.destroy();
            this.tableLayer = null;
        }
    },

    dealPoker: function(pokers, state) {

        //每次发牌都从空手牌开始，重复收到发牌消息时不能再往旧手牌上叠
        for (var i = 0; i < 3; i++) {
            this.players[i].cleanPokers();
        }

        //叫分阶段桌面上还有三张底牌；出牌阶段补发说明底牌已经被地主拿走
        if (state != PG.GameState.PLAYING) {
            for (var i = 0; i < 3; i++) {
                var p = new PG.Poker(this, 54, 54);
                this.game.world.add(p);
                this.tablePoker[i] = p.id;
                this.tablePoker[i + 3] = p;
            }
        }

        //另外两家的手牌只显示牌背，中途重连时按 17 张占位
        for (var i = 0; i < 17; i++) {
            this.players[2].pokerInHand.push(54);
            this.players[1].pokerInHand.push(54);
        }
        //自己的手牌张数以服务端下发的为准：叫分前 17 张，地主拿到底牌后 20 张
        for (var i = 0; i < pokers.length; i++) {
            this.players[0].pokerInHand.push(pokers[i]);
        }

        this.players[0].dealPoker();
        this.players[1].dealPoker();
        this.players[2].dealPoker();
        //this.game.time.events.add(1000, function() {
        //    this.send_message([PG.Protocol.REQ_CHEAT, this.players[1].uid]);
        //    this.send_message([PG.Protocol.REQ_CHEAT, this.players[2].uid]);
        //}, this);
    },
     
    showLastThreePoker: function() {
        for (var i = 0; i < 3; i++) {
            var pokerId = this.tablePoker[i];
            var p = this.tablePoker[i + 3];
            p.id = pokerId;
            p.frame = pokerId;
            this.game.add.tween(p).to({ x: this.game.world.width/2 + (i - 1) * 60}, 600, Phaser.Easing.Default, true);
        }
        this.pendingBottom = true;
        this.bottomTimer = this.game.time.events.add(1500, this.dealLastThreePoker, this);
    },

    //把三张底牌真正塞进地主手里；silent 表示底牌动画还没播完就被别人抢先出牌了
    dealLastThreePoker: function(silent) {
        if (!this.pendingBottom) {
            return;
        }
        this.pendingBottom = false;
        if (this.bottomTimer) {
            this.game.time.events.remove(this.bottomTimer);
            this.bottomTimer = null;
        }
        var seat = this.landlordSeat >= 0 ? this.landlordSeat : this.whoseTurn;
        var turnPlayer = this.players[seat];
        if (!turnPlayer || !this.tablePoker[3]) {
            return;
        }

        for (var i = 0; i < 3; i++) {
            var pid = this.tablePoker[i];
            var poker = this.tablePoker[i + 3];
            turnPlayer.pokerInHand.push(pid);
            turnPlayer.pushAPoker(poker);
        }
        turnPlayer.sortPoker();
        if (seat == 0) {
            turnPlayer.arrangePoker();
            for (var i = 0; i < 3; i++) {
                var p = this.tablePoker[i + 3];
                var tween = this.game.add.tween(p).to({y: this.game.world.height - PG.PH * 0.8 }, 400, Phaser.Easing.Default, true);
                function adjust(p) {
                    this.game.add.tween(p).to({y: this.game.world.height - PG.PH /2}, 400, Phaser.Easing.Default, true, 400);
                };
                tween.onComplete.add(adjust, this, p);
            }
        } else {
            var first = turnPlayer.findAPoker(54);
            for (var i = 0; i < 3; i++) {
                var p = this.tablePoker[i + 3];
                p.frame = 54;
                this.game.add.tween(p).to({ x: first.x, y: first.y}, 200, Phaser.Easing.Default, true);
            }
        }

        this.tablePoker = [];
        this.lastShotPlayer = turnPlayer;
        if (!silent && this.whoseTurn == 0) {
            this.startPlay();
        }
    },

    handleShotPoker: function(packet) {
        //底牌还在飞的时候地主就出牌了：先把底牌结清，否则桌上的牌和手牌张数都会错位
        if (this.pendingBottom) {
            this.dealLastThreePoker(true);
        }
        this.whoseTurn = this.uidToSeat(packet[1]);
        var turnPlayer = this.players[this.whoseTurn];
        var pokers = packet[2];
        if (pokers.length == 0) {
            this.players[this.whoseTurn].say("不出");
        } else {
            var pokersPic = {};
            pokers.sort(PG.Poker.comparePoker);
            var count= pokers.length;
            var gap = Math.min((this.game.world.width - PG.PW * 2) / count, PG.PW * 0.36);
            for (var i = 0; i < count; i++) {
                var p = turnPlayer.findAPoker(pokers[i]);
                p.id = pokers[i];
                p.frame = pokers[i];
                p.bringToTop();
                this.game.add.tween(p).to({ x: this.game.world.width/2 + (i - count/2) * gap, y: this.game.world.height * 0.4}, 500, Phaser.Easing.Default, true);

                turnPlayer.removeAPoker(pokers[i]);
                pokersPic[p.id] = p;
            }
        
            for (var i = 0; i < this.tablePoker.length; i++) {
                var p = this.tablePokerPic[this.tablePoker[i]];
                if (p) {
                    p.destroy();
                }
            }
            this.tablePoker = pokers;
            this.tablePokerPic = pokersPic;
            this.lastShotPlayer = turnPlayer;
            turnPlayer.arrangePoker();
        }
        if (turnPlayer.pokerInHand.length > 0) {
            this.whoseTurn = (this.whoseTurn + 1) % 3;
            if (this.whoseTurn == 0) {
                this.game.time.events.add(1000, this.startPlay, this);
            }
        }
    },

    //收起叫分按钮（叫分结束、开新局、离开牌桌时都要调）
    hideCallScore: function () {
        if (this.callLayer) {
            this.callLayer.destroy();
            this.callLayer = null;
        }
    },

    startCallScore: function(minscore) {
        //上一轮没点掉的按钮先收掉，否则会一局一局叠在桌面上
        this.hideCallScore();
        function btnTouch(btn) {
            this.send_message([PG.Protocol.REQ_CALL_SCORE, btn.score]);
            this.hideCallScore();
            var audio = this.game.add.audio('f_score_' + btn.score);
            audio.play();
        };

        if (this.whoseTurn == 0) {
            var step = this.game.world.width/6;
            var ss = [1.5, 1, 0.5, 0];
            var sx = this.game.world.width/2 - step * ss[minscore];
            var sy = this.game.world.height * 0.6;
            var group = this.game.add.group();
            var pass = this.game.make.button(sx, sy, 'ui_call_0', btnTouch, this);
            pass.anchor.set(0.5, 0);
            pass.score = 0;
            group.add(pass);
            sx += step;

            for (var i = minscore + 1; i <= 3; i++) {
                var call = this.game.make.button(sx, sy, 'ui_call_' + i, btnTouch, this);
                call.anchor.set(0.5, 0);
                call.score = i;
                group.add(call);
                sx += step;
            }
            this.callLayer = group;
        } else {
            // TODO show clock on player
        }
        
    },

    startPlay: function() {
        if (this.isLastShotPlayer()) {
            this.players[0].playPoker([]);
        } else {
            this.players[0].playPoker(this.tablePoker);
        }
    },

    finishPlay: function(pokers) {
        this.send_message([PG.Protocol.REQ_SHOT_POKER, pokers]);
    },

    isLastShotPlayer: function() {
        return this.players[this.whoseTurn] == this.lastShotPlayer;
    },

    createTableLayer: function (tables) {
        tables.push([-1, 0]);
        this.hideTableLayer();

        var group = this.game.add.group();
        this.tableLayer = group;
        this.game.world.bringToTop(group);
        var gc = this.game.make.graphics(0, 0);
        gc.beginFill(0x00000080);
        gc.endFill();
        group.add(gc);

        //牌桌是别的玩家后来建的，列表需要能手动刷新
        //放在标题下方居中，避开左右两角的对手头像
        var refresh = this.game.make.button(this.game.world.centerX, 68, 'ui_refresh', this.onRefreshTables, this);
        refresh.anchor.set(0.5, 0);
        group.add(refresh);

        for (var i = 0; i < tables.length; i++) {
            var sx = this.game.world.width * (i%6 + 1)/(6 + 1);
            var sy = 180 + Math.floor(i/6) * 100;
            var isNew = tables[i][0] == -1;

            //统一尺寸的牌桌底板 + 图标，新建牌桌用加号图标区分
            var table = this.game.make.button(sx, sy, 'ui_tile', this.onJoin, this);
            table.anchor.set(0.5, 1);
            table.scale.set(0.75);
            table.tableId = tables[i][0];
            group.add(table);

            var icon = this.game.make.image(sx, sy - 36, isNew ? 'icon_table_new' : 'icon_cards');
            icon.anchor.set(0.5, 0.5);
            icon.scale.set(38 / icon.width);
            group.add(icon);

            var label = isNew ? '新建房间' : ('牌桌 ' + tables[i][0] + ' · ' + tables[i][1] + ' 人');
            var text = this.game.make.text(sx, sy + 4, label, {font: "18px Arial", fill: "#dce8ee", align: "center"});
            text.anchor.set(0.5, 0);
            group.add(text);
        }
    },

    quitGame: function () {
        window.location.href = '/room/list';
    },

    createTitleBar: function() {
        var style = {font: "22px Arial", fill: "#fff", align: "center"};
        this.titleBar = this.game.add.text(this.game.world.centerX, 0, '房间:', style);
    },

    onRefreshTables: function () {
        this.send_message([PG.Protocol.REQ_TABLE_LIST]);
    },

    onJoin: function (btn) {
        if (btn.tableId == -1) {
            this.send_message([PG.Protocol.REQ_NEW_TABLE]);
        } else {
            this.send_message([PG.Protocol.REQ_JOIN_TABLE, btn.tableId]);
        }
        this.hideTableLayer();
    }
};






