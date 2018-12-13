function hasUserMedia() {
    navigator.getUserMedia = navigator.getUserMedia || navigator.msGetUserMedia || navigator.webkitGetUserMedia || navigator.mozGetUserMedia;
    return !!navigator.getUserMedia;
}

function hasRTCPeerConnection() {
    window.RTCPeerConnection = window.RTCPeerConnection || window.webkitRTCPeerConnection || window.mozRTCPeerConnection || window.msRTCPeerConnection;
    return !!window.RTCPeerConnection;
}

function SignalingChannel() {
    var ws = new WebSocket("ws://192.168.0.125:8082/chat/stream")
    ws.onclose = function(e) {
        console.log('closed', e)
    }
    ws.onopen = function(e) {
        console.log("auth ->")
        ws.send(JSON.stringify({
            type: "auth",
            from: username,
        }))
    }
    return ws
}


var yourVideo = document.getElementById("yours");
var theirVideo = document.getElementById("theirs");
var signalingChannel = SignalingChannel();
var username = "laoqiu";
var to = "test1";
var pc;
var startTimer;

startTimer = setTimeout(start, 3000);

signalingChannel.onmessage = function(e) {
    console.log("signalingChannel onmessage ->", e.data);
    var message = JSON.parse(e.data);
    if (message.type == "sdp") {
        pc.setRemoteDescription(new RTCSessionDescription(message.body), function(){
            if (pc.remoteDescription.type == "offer") {
                pc.createAnswer(localDescCreated, function(e){
                    console.log("!err", e)
                })
            }
        })
    }
    if (message.type == "candidate") {
        pc.addIceCandidate(new RTCIceCandidate(message.body));
    }
}

function localDescCreated(desc) {
    pc.setLocalDescription(desc, function () {
        signalingChannel.send(JSON.stringify({ type:"sdp", body: pc.localDescription }));
    }, function(e){
        console.log("setLocalDescription", e)
    });
 }

function start() {
    if (!hasRTCPeerConnection()) {
        alert("没有RTCPeerConnection API");
        return
    }

    var config = {
        'iceServers': [{ 'url': 'stun:stun.services.mozilla.com' }, { 'url': 'stun:stunserver.org' }, { 'url': 'stun:stun.l.google.com:19302' }]
    };

    pc = new RTCPeerConnection(config);

    pc.onicecandidate = function(e) {
        if (e.candidate) {
            signalingChannel.send(JSON.stringify({
                type: "candidate",
                to: to,
                body: e.candidate,
            }))
        }
    }

    pc.onnegotiationneeded = function() {
        pc.createOffer().then(offer =>{
            localDescCreated(offer)
        })
    }

    pc.onaddstream = function() {
        theirVideo.src = window.URL.createObjectURL(e.stream);
    }

    if (hasUserMedia()) {
        navigator.getUserMedia({ video: true, audio: false },
            stream => {
                yourVideo.src = window.URL.createObjectURL(stream);
                pc.addStream(stream)
            },
            err => {
                console.log(err);
            })
    } else {
        alert("没有userMedia API")
    }


}