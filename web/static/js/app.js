let stream = new MediaStream(); // 创建新的媒体流
let suuid = $('#suuid').val(); // 获取uuid

let config = {
  iceServers: [{
    urls: ["stun:stun.l.google.com:19302"]
  }]
};

const pc = new RTCPeerConnection(config);

// 当拿到编码信息以后自动触发的一个处理事件
pc.onnegotiationneeded = handleNegotiationNeededEvent;

let log = msg => {
  document.getElementById('div').innerHTML += msg + '<br>'
}

// 修改视频流的源流
pc.ontrack = function(event) {
  stream.addTrack(event.track);
  videoElem.srcObject = stream;
  log(event.streams.length + ' track is delivered')
}

pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)

// 当拿到编码信息以后自动触发的一个处理事件
async function handleNegotiationNeededEvent() {
  let offer = await pc.createOffer(); // 创建offer
  await pc.setLocalDescription(offer); // 设置描述
  getRemoteSdp(); // 获取远程的sdp
}

// 文档准备完毕
$(document).ready(function() {
  $('#' + suuid).addClass('active');
  getCodecInfo();
});


// 获取编码信息
function getCodecInfo() {
  $.get("../codec/" + suuid, function(data) {
    try {
      data = JSON.parse(data);
    } catch (e) {
      console.log(e);
    } finally {
      // 渲染编码信息
      $.each(data,function(index,value){
        pc.addTransceiver(value.Type, {
          'direction': 'sendrecv'
        })
      })
    }
  });
}

let sendChannel = null;

// 获取远程的sdp
function getRemoteSdp() {
  $.post("../receiver/"+ suuid, {
    suuid: suuid,
    data: btoa(pc.localDescription.sdp)
  }, function(data) {
    try {
      // 设置远程描述
      pc.setRemoteDescription(new RTCSessionDescription({
        type: 'answer',
        sdp: atob(data)
      }))
    } catch (e) {
      console.warn(e);
    }
  });
}
