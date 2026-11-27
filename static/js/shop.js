//外观商店：购买 / 穿戴，成功后原地重绘按钮，不整页刷新
(function () {
    var items = window.PG_SHOP ? window.PG_SHOP.items : [];
    var coinEl = document.getElementById('wallet-coin');
    var errorEl = document.getElementById('shop-error');

    function setError(msg) {
        if (!errorEl) return;
        errorEl.textContent = msg || '';
    }

    function findItem(id) {
        for (var i = 0; i < items.length; i++) {
            if (items[i].id === id) return items[i];
        }
        return null;
    }

    function findCard(id) {
        return document.querySelector('.shop-card[data-item="' + id + '"]');
    }

    //根据商品状态重绘一张卡片的按钮
    function renderCard(item) {
        var card = findCard(item.id);
        if (!card) return;
        var slot = card.querySelector('.action-slot');
        if (!slot) return;
        card.className = 'shop-card' + (item.equipped ? ' equipped' : '');
        if (item.equipped) {
            slot.innerHTML = '<button type="button" class="equipped" disabled>使用中</button>';
        } else if (item.owned) {
            slot.innerHTML = '<button type="button" class="equip" data-item="' + item.id + '">使用</button>';
        } else {
            slot.innerHTML = '<button type="button" class="buy" data-item="' + item.id + '">购买</button>';
        }
    }

    function applyState(payload) {
        if (typeof payload.coin === 'number' && coinEl) {
            coinEl.textContent = payload.coin;
        }
        if (payload.items) {
            items = payload.items;
            for (var i = 0; i < items.length; i++) {
                renderCard(items[i]);
            }
        }
    }

    function request(url, itemId) {
        var body = new URLSearchParams();
        body.set('item_id', itemId);
        setError('');
        return fetch(url, {
            method: 'POST',
            headers: {'Content-Type': 'application/x-www-form-urlencoded;charset=UTF-8'},
            body: body.toString(),
            credentials: 'same-origin'
        }).then(function (response) {
            return response.json().then(function (payload) {
                return {ok: response.ok, payload: payload};
            });
        }).then(function (result) {
            if (!result.ok || !result.payload.ok) {
                setError(result.payload.error || '操作失败，请稍后再试');
                return;
            }
            applyState(result.payload);
        }).catch(function () {
            setError('网络异常，请稍后再试');
        });
    }

    document.addEventListener('click', function (event) {
        var button = event.target;
        if (!button || button.tagName !== 'BUTTON') return;
        if (button.classList.contains('buy')) {
            request('/api/shop/buy', button.getAttribute('data-item'));
        } else if (button.classList.contains('equip')) {
            request('/api/shop/equip', button.getAttribute('data-item'));
        }
    });

    //首屏也要把按钮补齐（模板只写了价格/默认标签）
    for (var i = 0; i < items.length; i++) {
        renderCard(items[i]);
    }
})();