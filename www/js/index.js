"use strict";

var kDatabaseName = 'WebRTC-Database';
var gDatabase = null;

function openDatabase() {
    return new Promise(function(resolve, reject) {
        var reqOpen = indexedDB.open(kDatabaseName);
        reqOpen.onupgradeneeded = function() {
            var db = reqOpen.result;
            var certStore = db.createObjectStore('certificates', { keyPath: 'id' });
        };
        reqOpen.onsuccess = function() {
            gDatabase = reqOpen.result;
            resolve(reqOpen.result)
        }
        reqOpen.onerror = function() {
            reject()
        }
    });
}

function saveCertificate_(certificate) {
    return new Promise(function(resolve, reject) {
        var certTrans = gDatabase.transaction('certificates', 'readwrite');
        var certStore = certTrans.objectStore('certificates');
        var certPut = certStore.put({
            id:0,
            cert:certificate
        });
        certPut.onsuccess = function() {
            resolve();
        };
        certPut.onerror = function() {
            reject(certPut.error);
        };
    });
}

function loadCertificate_() {
    return new Promise(function(resolve, reject) {
        var certTrans = gDatabase.transaction('certificates', 'readonly');
        var certStore = certTrans.objectStore('certificates');
        var reqGet = certStore.get(0);
        reqGet.onsuccess = function() {
            var match = reqGet.result;
            if (match !== undefined) {
                resolve(match.cert);
            } else {
                resolve(null);
            }
        };
        reqGet.onerror = function() {
            reject(reqGet.error);
        };
    });
}


// Start communication with signaling server.
// Single RT with the offer, will receive the response.
async function startSignal() {
    await openDatabase()
    let cert = await loadCertificate_()

    console.log("Saved: ", cert)
    if (cert === null || cert === '') {
        cert = await RTCPeerConnection.generateCertificate({
            name: "ECDSA",
            namedCurve: "P-256"
        })

        saveCertificate_(cert)
    }

    console.log("Cert", cert.getFingerprints())

    const configuration = {
        'certificates': [cert],
        'iceServers': [] //{'urls': 'stun:stun.l.google.com:19302'}]
    }
    const peerConnection = new RTCPeerConnection(configuration);
    const dataChannel = peerConnection.createDataChannel('ctl');

    peerConnection.addEventListener('datachannel', event => {
        const dataChannel = event.channel;
        console.log("datachannel event", event, dataChannel)
    });
    // Append new messages to the box of incoming messages
    dataChannel.addEventListener('message', event => {
        const message = event.data;
        console.log(message, event)
        dataChannel.send("pong")
    });
    dataChannel.addEventListener('close', event => {
        console.log(event)
    });
    dataChannel.addEventListener('open', event => {
        console.log(event)
        dataChannel.send("open")
    });

    peerConnection.onicecandidate = function (ev) {
        console.log("ICE ", ev.candidate);
    }
    peerConnection.oniceconnectionstatechange = function (ev) {
        console.log("ICE ConnState", ev)
    }
    peerConnection.onconnectionstatechange = function (ev) {
        console.log("ICE ConnectionStateChange", ev)
    }
    peerConnection.onicegatheringstatechange = function (ev) {
        console.log("ICE Gatherstate", ev)
    }
    peerConnection.onsignalingstatechange = function (ev) {
        console.log("ICE Signalingstatechange", ev)
    }

    // Causes ICE to start iceing. This is where we should wait before creating offer
    const offer = await peerConnection.createOffer();

    await peerConnection.setLocalDescription(offer);

    console.log(offer)
    console.log("New", peerConnection.currentLocalDescription)

    // TODO: a single RPC with offer as param, remote as response.
    fetch("/wrtc/direct/", {
        method: "POST",
        body:  JSON.stringify(offer),
    }).then(function (res) {
        return res.json();
    }).then(function (message) {
        //if (message.a) {
            const remoteDesc = new RTCSessionDescription(message);
            console.log("Got answer: ", message)
            peerConnection.setRemoteDescription(remoteDesc);
        //}

    });
}

function initDC(dataChannel) {
    const messageBox = document.querySelector('#messageBox');
    const sendButton = document.querySelector('#sendButton');
    const incomingMessages = document.querySelector('#incomingMessages');

    sendButton.addEventListener('click', event => {
        const message = messageBox.textContent;
        dataChannel.send(message);
    })

    // Append new messages to the box of incoming messages
    dataChannel.addEventListener('message', event => {
        const message = event.data;
        incomingMessages.textContent += message + '\n';
    });

    // Enable textarea and button when opened
    dataChannel.addEventListener('open', event => {
        messageBox.disabled = false;
        messageBox.focus();
        sendButton.disabled = false;
    });

    // Disable input when closed
    dataChannel.addEventListener('close', event => {
        messageBox.disabled = false;
        sendButton.disabled = false;
    });


}

function ice(peerConnection) {
    // Listen for local ICE candidates on the local RTCPeerConnection
    peerConnection.addEventListener('icecandidate', event => {
        if (event.candidate) {
            signalingChannel.send({'new-ice-candidate': event.candidate});
        }
    });

    // Listen for remote ICE candidates and add them to the local RTCPeerConnection
    signalingChannel.addEventListener('message', async message => {
        if (message.iceCandidate) {
            try {
                await peerConnection.addIceCandidate(message.iceCandidate);
            } catch (e) {
                console.error('Error adding received ice candidate', e);
            }
        }
    });

    // Listen for connectionstatechange on the local RTCPeerConnection
    peerConnection.addEventListener('connectionstatechange', event => {
        if (peerConnection.connectionState === 'connected') {
            // Peers connected!
        }
    });
}

function wifiHandler() {
    $("#do_sync").click(function () {
        fetch("debug/scan?s=1").then(function (res) {
            return res.json();
        }).then(function (json) {
            // TODO: update just the wifi table. Reload also refreshes,
            // since visibledevices is returned.
            //location.reload();
        });
    });

    $("#apcheck").change(function () {
        if ($("#apcheck")[0].checked) {
            fetch("dmesh/uds?q=/wifi/p2p&ap=1").then(function (json) {
                //    location.reload();
            });
        } else {
            fetch("dmesh/uds?q=/wifi/p2p&ap=0").then(function (json) {
                //    location.reload();
            });
        }
    });
    $("#autocon").click(function () {
        fetch("wifi/con").then(function (json) {
            location.reload();
        });
    });
    $("#mc").click(function () {
        fetch("dmesh/mc").then(function (json) {
            location.reload();
        });
    });
    $("#nanping").click(function () {
        fetch("dmesh/uds?q=/wifi/nan/ping").then(function (json) {
            location.reload();
        });
    });
    $("#nanoff").click(function () {
        fetch("dmesh/uds?q=/wifi/nan/stop").then(function (json) {
            location.reload();
        });
    });
    $("#nanon").click(function () {
        fetch("dmesh/uds?q=/wifi/nan/start").then(function (json) {
            location.reload();
        });
    });

    fetch("dmesh/ll/if").then(function (res) {
        return res.json();
    }).then(function (json) {
    })

    eventsHandler()

}

function eventsHandler() {
    //$("#do_sync").click(function () {
    //updateEV()
    //});

    updateEV();

    var evtSource = new EventSource("debug/eventss");
    evtSource.onmessage = function (e) {
        console.log("XXX EVENT", e)
        onEvent(JSON.parse(e.data))
    }
    evtSource.onerror = function (e) {
        console.log("event err", e)
    }
    evtSource.onopen = function (e) {
        console.log("event open", e)
    }
    //evtSource.addEventListener("wifi/scan", onEvent)
}

function updateTCP() {
    fetch("/dmesh/tcpa").then(function (res) {
        if (console != null) {
            console.log(res);
        }
        return res.json();
    }).then(function (json) {
        $("#tcptable tbody").remove();
        let t = $("#tcptable").get()[0]

        $.each(json, function (i, ip) {
            let row = t.insertRow(-1);

            let cell = $("<td />");
            cell.html("<div>" + i + "</div>");
            $(row).append(cell);

            cell = $("<td />");
            cell.html("<div>" + ip.Dest + "</div>");
            $(row).append(cell);

            cell = $("<td />");
            cell.html("<div>" + JSON.stringify(ip) + "</div>");
            $(row).append(cell);

            cell = $("<td />");
            cell.html("<div>" + ip.RcvdBytes + "</div>");
            $(row).append(cell);

            cell = $("<td />");
            cell.html("<div>" + ip.Last + "</div>");
            $(row).append(cell);

            //cell.title = JSON.stringify(ip);
            //cell.tooltip();
            $(row).append(cell);


        });
    });
}


function updateUDP() {
    fetch("/dmesh/udp").then(function (res) {
        if (console != null) {
            console.log(res);
        }
        return res.json();
    }).then(function (json) {
        $("#tcptable tbody").remove();
        let t = $("#udptable").get()[0]

        $.each(json, function (i, ip) {
            let row = t.insertRow(-1);

            let cell = $("<td />");
            cell.html("<div>" + i + "</div>");
            $(row).append(cell);

            cell = $("<td />");
            cell.html("<div>" + ip.Count + "</div>");
            $(row).append(cell);

            cell = $("<td />");
            cell.html("<div>" + ip.SentBytes + "</div>");
            $(row).append(cell);

            cell = $("<td />");
            cell.html("<div>" + ip.RcvdBytes + "</div>");
            $(row).append(cell);

            cell = $("<td />");
            cell.html("<div>" + ip.Last + "</div>");
            $(row).append(cell);

            //cell.title = JSON.stringify(ip);
            //cell.tooltip();
            $(row).append(cell);
        });
    });
}


function onEvent(ev) {
    if (ev.to == undefined) {
        return
    }
    let row = "<tr>";

    row += "<td>" + ev.to +
        "<br>" + ev.from +
        "</br>" + ev.time +
        "</td>";

    row += "<td>";

    $.each(ev.meta, function (k, v) {
        if (false && ev.to.startsWith("/SYNC") && k == "Node") {
            let vv = JSON.parse(v)

            row += " NodeUA = " + vv.Reg.UA + "</br>";
            row += " NodeVIP = " + vv.vip + "</br>";

            row += " NodeGW = " + JSON.stringify(vv.gw) + "</br>";
            if (vv.Reg.nodes != undefined) {
                $.each(vv.Reg.nodes, function (k, v) {
                    row += " Node " + k + " = " + JSON.stringify(v) + "</br>";
                })
            }

            if (vv.Reg.wifi != undefined) {
                row += " Wifi = " + JSON.stringify(vv.Reg.wifi) + "</br>";
                // $.each(vv.Reg.wifi.P2P, function (k, v) {
                //     row += " Wifi " + k + " = " + v.Build + " " + v.Name + " " + v.Net + " " + v.SSID + " " + v.Pass + "</br>";
                // })
            }

        } else {
            row += k + " = " + v + "</br>";
        }

    })
    if (ev.path) {
        row += "<br/>" + ev.path;
    }
    row += "</td>";
    row += "</tr>";

    // $.each(ip.Value.Meta, function (k, v) {
    //     txt += k + "= <pre class='prettyprint'><code>"  + v + "</code></pre>";
    // })

    $(row).prependTo('#evtable tbody');

}

function updateEV() {
    fetch("debug/eventslog").then(function (res) {
        if (console != null) {
        }
        return res.json();
    }).then(function (json) {
        $("#evptable tbody").remove();
        //let t = $("#evtable").get()[0]

        console.log("events1", json)
        if (json) {
            $.each(json, function (i, ip) {
                onEvent(ip);

            });
        }
    });
}


function updateIP6() {
    // fetch("/dmesh/ip6").then(function (res) {
    //     console.log(res);
    //     return res.json();
    // }).then(function (json) {
    //     $("#ip6table tr").remove();
    //     let t = $("#ip6table").get()[0]
    //
    //     $.each(json, function (i, ip) {
    //         let row = t.insertRow(-1);
    //         let cell = $("<td />");
    //         cell.html("<div>" + ip.UserAgent + "</div>");
    //         $(row).append(cell);
    //
    //         cell = $("<td />");
    //         cell.html("<div>" + ip.GW.IP + "</div>");
    //         $(row).append(cell);
    //
    //         cell = $("<td/>");
    //         cell.html("<div  data-toggle='tooltip'>" + ip.LastSeen + "</div>");
    //         cell.title = JSON.stringify(ip);
    //         cell.tooltip();
    //         $(row).append(cell);
    //     });
    // });

    /*
    fetch("/dmesh/tcp").then(function (res) {
        console.log(res);
        return res.json();
    }).then(function (json) {
        $("#tcptable tr").remove();
        let t = $("#ip6table").get()[0]

        $.each(json, function (i, ip) {
            let row = t.insertRow(-1);
            let cell = $("<td />");

            cell.html("<div>" + i + "</div>");
            $(row).append(cell);

            cell = $("<td />");
            cell.html("<div>" + ip.Count + "</div>");
            $(row).append(cell);

            cell = $("<td />");
            cell.html("<div>" + ip.SentBytes + "</div>");
            $(row).append(cell);

            cell = $("<td />");
            cell.html("<div>" + ip.RcvdBytes + "</div>");
            $(row).append(cell);

            cell = $("<td />");
            cell.html("<div>" + ip.Last + "</div>");
            $(row).append(cell);


            cell.title = JSON.stringify(ip);
            //cell.tooltip();
            $(row).append(cell);
        });
    });
    */

}

window.addEventListener("message", function (e) {
    console.log("Main onmessage " + e);
    if (e.data.log) {
        console.log(e.data.log);
    } else {

    }
}, false);

$('#svc').click(function (event) {
    // Remember the link href
    let href = this.href;

    // Don't follow the link
    event.preventDefault();

    fetch(href).then(function (res) {
        console.log(res.json());
    });
});

document.addEventListener("DOMContentLoaded", function(event) {
    //do work
    updateTCP();
    startSignal();

});

// $(document).ready(function () {
//     $("#do_sync").click(function () {
//         console.log("Sync")
//         fetch("/dmesh/rd").then(function (res) {
//             updateIP6()
//         })
//     });
//
//     updateIP6();
// })

