PG = {
    music: null,
    playerInfo: {},
    orientated: false
};

PG.getCookie = function (name) {
    var r = document.cookie.match("\\b" + name + "=([^;]*)\\b");
    return r ? r[1] : undefined;
};

PG.PW = 90;
PG.PH = 120;

PG.Boot = {
    preload: function () {
        this.load.image('preloaderBar', '/static/i/preload.png');
    },
    create: function () {
        this.input.maxPointers = 1;
        this.stage.disableVisibilityChange = true;
        this.scale.scaleMode = Phaser.ScaleManager.SHOW_ALL;
        this.scale.enterIncorrectOrientation.add(this.enterIncorrectOrientation, this);
        this.scale.leaveIncorrectOrientation.add(this.leaveIncorrectOrientation, this);
        this.onSizeChange();
        this.state.start('Preloader');
    },
    onSizeChange: function () {
        this.scale.minWidth = 480;
        this.scale.minHeight = 270;
        var device = this.game.device;
        if (device.android || device.iOS) {
            this.scale.maxWidth = window.innerWidth;
            this.scale.maxHeight = window.innerHeight;
        } else {
            this.scale.maxWidth = 960;
            this.scale.maxHeight = 540;
        }
        this.scale.pageAlignHorizontally = true;
        this.scale.pageAlignVertically = true;
        this.scale.forceOrientation(true);
    },
    enterIncorrectOrientation: function () {
        PG.orientated = false;
        document.getElementById('orientation').style.display = 'block';
    },
    leaveIncorrectOrientation: function () {
        PG.orientated = true;
        document.getElementById('orientation').style.display = 'none';
    }
};

PG.Preloader = {

    preload: function () {
        this.preloadBar = this.game.add.sprite(120, 200, 'preloaderBar');
        this.load.setPreloadSprite(this.preloadBar);

        this.load.audio('music_room', '/static/audio/bg_room.mp3');
        this.load.audio('music_game', '/static/audio/bg_game.ogg');
        this.load.audio('music_deal', '/static/audio/deal.mp3');
        this.load.audio('music_win', '/static/audio/end_win.mp3');
        this.load.audio('music_lose', '/static/audio/end_lose.mp3');
        this.load.audio('f_score_0', '/static/audio/f_score_0.mp3');
        this.load.audio('f_score_1', '/static/audio/f_score_1.mp3');
        this.load.audio('f_score_2', '/static/audio/f_score_2.mp3');
        this.load.audio('f_score_3', '/static/audio/f_score_3.mp3');
        //统一图标包：头像、操作按钮、功能入口都从这里取
        this.load.image('avatar_default', '/static/i/avatars/default.png');
        this.load.image('avatar_farmer', '/static/i/avatars/farmer.png');
        this.load.image('avatar_landlord', '/static/i/avatars/landlord.png');
        this.load.image('icon_coin', '/static/i/icons/coin.png');
        this.load.image('icon_cards', '/static/i/icons/cards.png');
        this.load.image('icon_table_new', '/static/i/icons/table-plus.png');
        this.load.image('icon_trophy', '/static/i/icons/trophy.png');
        this.load.image('ui_action_pass', '/static/i/ui/action-pass.png');
        this.load.image('ui_action_hint', '/static/i/ui/action-hint.png');
        this.load.image('ui_action_play', '/static/i/ui/action-play.png');
        this.load.image('ui_call_0', '/static/i/ui/call-0.png');
        this.load.image('ui_call_1', '/static/i/ui/call-1.png');
        this.load.image('ui_call_2', '/static/i/ui/call-2.png');
        this.load.image('ui_call_3', '/static/i/ui/call-3.png');
        this.load.image('ui_refresh', '/static/i/ui/refresh.png');
        this.load.image('ui_addrobot', '/static/i/ui/addrobot.png');
        this.load.image('ui_tile', '/static/i/ui/tile.png');
        //背景和牌面按账号当前穿戴加载；没有配置时用默认外观
        var cardAsset = (PG.playerInfo && PG.playerInfo.card) || '/static/i/poker.png';
        var bgAsset = (PG.playerInfo && PG.playerInfo.bg) || '/static/i/bg/classic.png';
        this.load.image('tablebg', bgAsset);
        this.load.spritesheet('poker', cardAsset, 90, 120);
        this.load.json('rule', '/static/rule.json');
    },

    create: function () {
        PG.RuleList = this.cache.getJSON('rule');
        //房间号来自页面路由 /room/{rid}，由模板写进 user 里
        var roomId = (PG.playerInfo && PG.playerInfo.room_id) || PG.roomId || 1;
        PG.music = this.game.add.audio('music_room');
        PG.music.loop = true;
        PG.music.loopFull();
        PG.music.play();
        this.state.start('Game', true, false, roomId);
    }
};
