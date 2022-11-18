import {
  ConnectionQuality,
  ConnectionState,
  DataPacket_Kind,
  DisconnectReason,
  LocalParticipant,
  LogLevel,
  MediaDeviceFailure,
  Participant,
  ParticipantEvent,
  RemoteParticipant,
  RemoteTrackPublication,
  RemoteVideoTrack,
  Room,
  RoomConnectOptions,
  RoomEvent,
  RoomOptions,
  setLogLevel,
  Track,
  TrackPublication,
  VideoCaptureOptions,
  VideoCodec,
  VideoPresets,
  VideoQuality,
} from '../src/index';
import Dropzone from "dropzone";
import Swal from "sweetalert2";
import SimpleBar from "simplebar";
import { Buffer } from "buffer";
import fscreen from "fscreen";


const $ = (id: string) => document.getElementById(id);
const API_URL = "https://meeting.fptonline.net";
const WSS_URL = "wss://wss-meeting.fptonline.net";
const isMobile = /Mobile|Android|iP(hone|od)|IEMobile|BlackBerry|Kindle|Silk-Accelerated|(hpw|web)OS|Opera M(obi|ini)/.test(navigator.userAgent);
let readyRecieveNotify = false;

// register service worker
if ("serviceWorker" in navigator) {
  window.addEventListener("load", function() {
      navigator.serviceWorker.register("/sw.js").then(function(registration) {
          // console.log("ServiceWorker registration successful with scope: ", registration.scope);
      }, function(err) {
          // console.log("ServiceWorker registration failed: ", err);
      });
  });       
}

// PWA
// let defferedPrompt: any;
// window.addEventListener('beforeinstallprompt', event => {
//     event.preventDefault();
//     defferedPrompt = event;
//     Swal.fire({
//       title: 'Install FO Meeting app',
//       text: "Click install in the next prompt to add a shortcut to your desktop screen.",
//       showCancelButton: true,
//       confirmButtonColor: '#3085d6',
//       confirmButtonText: 'Yes',
//       cancelButtonText: 'Not now',
//       heightAuto: false,
//       background: '#292c3d',
//     }).then((result) => {
//       if (result.isConfirmed) {
//         defferedPrompt.prompt();
//         defferedPrompt.userChoice.then((choice: any) => {
//             if(choice.outcome === 'accepted'){
//               console.log('user accepted the prompt')
//             }
//             defferedPrompt = null;
//         });
//       }
//     })
// });

// upload avatar
let localAvatar: string|null;
let localName: string|null;
const dropzoneElm = <HTMLElement>document.querySelector(".dz-upload-avatar");
if (dropzoneElm) {
  const avatarUploader = new Dropzone(dropzoneElm, {
      url: "/upload-avatar",
      maxFiles: 1,
      uploadMultiple: false,
      acceptedFiles: "image/*",
      resizeWidth: 500,
      resizeHeight: 500,
      resizeQuality: 1,
      createImageThumbnails: false,
      previewsContainer: false,
      clickable: ".box-upload-file"
  });
  avatarUploader.on("error", (file: any, message: any) => {
    avatarUploader.removeAllFiles(true);
    Swal.fire({
      title: 'Error',
      text: 'Upload error. Please try again',
      showCancelButton: false,
      confirmButtonColor: '#0E78F9',
      cancelButtonColor: '#292C3D',
      confirmButtonText: 'Ok',
      showCloseButton: true,
      heightAuto: false,
      background: '#292c3d',
    });
    console.log(`upload error: ${file.name} - ${message}`);
  });
  avatarUploader.on("success", (file: any) => {
      console.log(file);
      let res = JSON.parse(file.xhr?.response);
      if (res.status == 1) {
        localAvatar = res.message;
        const avatar = <HTMLDivElement>document.querySelector("#joinPage .box-avatar");
        avatar.innerHTML = `<img src="${res.message}" alt="">`;

        localStorage.setItem("avatar", res.message);
      } else {
        Swal.fire({
          title: 'Error',
          text: 'Upload error. Please try again',
          showCancelButton: false,
          confirmButtonColor: '#0E78F9',
          cancelButtonColor: '#292C3D',
          confirmButtonText: 'Ok',
          showCloseButton: true,
          heightAuto: false,
          background: '#292c3d',
        });
        console.log("upload error:", res);
      }
      avatarUploader.removeAllFiles(true);
  });
}
localAvatar = localStorage.getItem("avatar");
if (localAvatar) {
    const avatar = <HTMLDivElement>document.querySelector("#joinPage .box-avatar");
    if (avatar) {
      avatar.innerHTML = `<img src="${localAvatar}" alt="">`;
    }
    localAvatar = localAvatar;
}
localName = localStorage.getItem("name");

// join page
let joinMic = false;
let joinCam = false;
let joinMicButton = <HTMLButtonElement>$("btnJoinMic");
let joinCamButton = <HTMLButtonElement>$("btnJoinCam");
joinMicButton?.addEventListener("click", (e) => {
  if (joinMicButton && joinMicButton.innerHTML == '<i class="icon-Voiceoff"></i>') {
    joinMic = true;
    joinMicButton.innerHTML = '<i class="icon-Voice"></i>';
  } else {
    joinMic = false;
    joinMicButton.innerHTML = '<i class="icon-Voiceoff"></i>';
  }
});
joinCamButton?.addEventListener("click", (e) => {
  if (joinCamButton && joinCamButton.innerHTML == '<i class="icon-Video"></i>') {
    joinCam = false;
    joinCamButton.innerHTML = '<i class="icon-Videooff"></i>';
  } else {
    joinCam = true;
    joinCamButton.innerHTML = '<i class="icon-Video"></i>';
  }
});

// room
const state = {
  isFrontFacing: false,
  encoder: new TextEncoder(),
  decoder: new TextDecoder(),
  defaultDevices: new Map<MediaDeviceKind, string>(),
  bitrateInterval: undefined as any,
  layout: 1,
  raiseHand: false
};
let currentRoom: Room | undefined;
let startTime: number;

const nameInputElm = <HTMLInputElement>document.querySelector('input[name="name"]');
const roomInputElm = <HTMLInputElement>document.querySelector('input[name="room"]');
const roomPasscodeElm = <HTMLInputElement>document.querySelector('input[name="room-passcode"]');
if (localName) {
  if (nameInputElm) {
    nameInputElm.value = localName;
  }
  if (roomInputElm) {
    roomInputElm.focus();
  } else if (roomPasscodeElm) {
    roomPasscodeElm.focus();
  } else {
    (<HTMLButtonElement>$("btnJoin"))?.focus();
  }
}
nameInputElm?.addEventListener("keyup", function (e: any) {
  if (e.key === 'Enter' || e.keyCode === 13) {
    appActions.connectWithFormInput();
  }
});
roomInputElm?.addEventListener("keyup", function (e: any) {
  if (e.key === 'Enter' || e.keyCode === 13) {
    appActions.connectWithFormInput();
  }
});
roomPasscodeElm?.addEventListener("keyup", function (e: any) {
  if (e.key === 'Enter' || e.keyCode === 13) {
    appActions.connectWithFormInput();
  }
});

// handles actions from the HTML
const appActions = {
  connectWithFormInput: async () => {
    const btnJoin = <HTMLButtonElement>$("btnJoin");
    const storedRoom = location.pathname.replace(/^\//g, "");
    const userName = nameInputElm.value;
    const roomName = roomInputElm ? roomInputElm.value : storedRoom;
    
    btnJoin.setAttribute('disabled', 'true');
    btnJoin.innerHTML = "JOINING...";
    if (!roomName || !userName) {
      btnJoin.innerHTML = "JOIN";
      btnJoin.removeAttribute('disabled');
      Swal.fire({
        title: 'Notify',
        text: 'Please provide your name and room name',
        showCancelButton: false,
        confirmButtonColor: '#0E78F9',
        cancelButtonColor: '#292C3D',
        confirmButtonText: 'Ok',
        showCloseButton: true,
        heightAuto: false,
        background: '#292c3d',
      });
      return;
    }
    localStorage.setItem("name", userName);

    let dataPost: any = {
      "user_name": userName,
      "avatar": localAvatar,
      "room": roomName,
      "create": false
    };
    if (storedRoom == "") {
      dataPost.create = true;
    }
    const roomPasscode = roomPasscodeElm?.value;
    if (roomPasscode && roomPasscode != "") {
      dataPost.passcode = roomPasscode;
    }
    const uii = localStorage.getItem("uii");
    if (uii) {
      dataPost.uii = uii;
    }
    const token = await fetch(`${API_URL}/api/get-join-token`, {
      method: 'POST',
      cache: 'no-cache',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(dataPost)
    }).then(
      async (result) => {
        const {
          token,
          room,
          uii,
          error,
          message
        } = await result.json();

        if (error == 1) {
          Swal.fire({
            title: 'Notify',
            text: message,
            showCancelButton: false,
            confirmButtonColor: '#0E78F9',
            cancelButtonColor: '#292C3D',
            confirmButtonText: 'Ok',
            showCloseButton: true,
            heightAuto: false,
            background: '#292c3d',
          });
          return;
        }
        
        console.log("get-join-token", token);
        window.history.replaceState(null, '', `/${room}`);
        if (uii) {
          localStorage.setItem("uii", uii);
        }

        return token;
      }
    ).catch(() => {
      btnJoin.innerHTML = "JOIN";
      btnJoin.removeAttribute('disabled');
    });

    if (token == null) {
      btnJoin.innerHTML = "JOIN";
      btnJoin.removeAttribute('disabled');
      return;
    }

    const simulcast = true;
    const forceTURN = false;
    const shouldPublish = true;
    const preferredCodec = "" as VideoCodec;

    setLogLevel(LogLevel.debug);

    const roomOpts: RoomOptions = {
      // adaptiveStream
      adaptiveStream: true,
      dynacast: true,
      publishDefaults: {
        simulcast,
        videoSimulcastLayers: [VideoPresets.h90, VideoPresets.h216],
        videoCodec: preferredCodec || 'vp8',
      },
      videoCaptureDefaults: {
        resolution: VideoPresets.h720.resolution,
      },
    };

    const connectOpts: RoomConnectOptions = {
      autoSubscribe: true,
    };
    if (forceTURN) {
      connectOpts.rtcConfig = {
        iceTransportPolicy: 'relay',
      };
    }
    await appActions.connectToRoom(WSS_URL, token, roomOpts, connectOpts, shouldPublish);
    
    state.bitrateInterval = setInterval(renderBitrate, 1000);
  },

  connectToRoom: async (
    url: string,
    token: string,
    roomOptions?: RoomOptions,
    connectOptions?: RoomConnectOptions,
    shouldPublish?: boolean,
  ): Promise<Room | undefined> => {
    const room = new Room(roomOptions);

    startTime = Date.now();
    await room.prepareConnection(url);
    const prewarmTime = Date.now() - startTime;
    appendLog(`prewarmed connection in ${prewarmTime}ms`);

    room
      .on(RoomEvent.ParticipantConnected, participantConnected)
      .on(RoomEvent.ParticipantDisconnected, participantDisconnected)
      .on(RoomEvent.DataReceived, handleData)
      .on(RoomEvent.Disconnected, handleRoomDisconnect)
      .on(RoomEvent.Reconnecting, () => appendLog('Reconnecting to room'))
      .on(RoomEvent.Reconnected, () => {
        appendLog('Successfully reconnected. server', room.engine.connectedServerAddress);
      })
      .on(RoomEvent.LocalTrackPublished, () => {
        renderParticipant(room.localParticipant);
        updateButtonsForPublishState();
        renderScreenShare(room);
      })
      .on(RoomEvent.LocalTrackUnpublished, () => {
        renderParticipant(room.localParticipant);
        updateButtonsForPublishState();
        renderScreenShare(room);

        let screenShare = (<HTMLDivElement>$(`screenshare-wrapper-${room.localParticipant.identity}`));
        if (screenShare) {
          screenShare.remove();
          handleLayouts();
        }
      })
      .on(RoomEvent.DataReceived, (data) => {
        appendLog('new data received for room', data);

        const jsonString = Buffer.from(data).toString('utf8');
        try {
          const parsedData = JSON.parse(jsonString);
          if (parsedData) {
            if (typeof parsedData.toggle_camera == "boolean") {
              appActions.toggleVideo();
            }
            if (typeof parsedData.off_screenshare == "boolean") {
              appActions.shareScreen();
            }
          }
        } catch (ex) {

        }
      })
      .on(RoomEvent.RoomMetadataChanged, (metadata) => {
        appendLog('new metadata for room', metadata);
      })
      .on(RoomEvent.MediaDevicesChanged, handleDevicesChanged)
      .on(RoomEvent.AudioPlaybackStatusChanged, () => {
        if (room.canPlaybackAudio) {
          $('start-audio-button')?.setAttribute('disabled', 'true');
        } else {
          $('start-audio-button')?.removeAttribute('disabled');
        }
      })
      .on(RoomEvent.MediaDevicesError, (e: Error) => {
        const failure = MediaDeviceFailure.getFailure(e);
        appendLog('media device failure', failure);
      })
      .on(
        RoomEvent.ConnectionQualityChanged,
        (quality: ConnectionQuality, participant?: Participant) => {
          appendLog('connection quality changed', participant?.name, quality);
        },
      )
      .on(RoomEvent.TrackSubscribed, (_1, pub, participant) => {
        appendLog('subscribed to track', pub.trackSid, participant.identity);
        renderParticipant(participant);
        renderScreenShare(room);
      })
      .on(RoomEvent.TrackUnsubscribed, (_, pub, participant) => {
        appendLog('unsubscribed from track', pub.trackSid);
        renderParticipant(participant);
        renderScreenShare(room);

        let screenShare = (<HTMLDivElement>$(`screenshare-wrapper-${participant.identity}`));
        if (screenShare) {
          screenShare.remove();
          handleLayouts();
        }
      })
      .on(RoomEvent.SignalConnected, async () => {
        const signalConnectionTime = Date.now() - startTime;
        appendLog(`signal connection established in ${signalConnectionTime}ms`);
        if (shouldPublish) {
          await Promise.all([
            room.localParticipant.setCameraEnabled(joinCam),
            room.localParticipant.setMicrophoneEnabled(joinMic),
          ]);
          updateButtonsForPublishState();
          startCountTimer();
          if (joinMic) {
            acquireDeviceList("audio-input");
            acquireDeviceList("audio-output");
          }
          if (joinCam) {
            acquireDeviceList("video-input");
          }
        }
      });

    try {
      await room.connect(url, token, connectOptions);
      const elapsed = Date.now() - startTime;
      appendLog(
        `successfully connected to ${room.name} in ${Math.round(elapsed)}ms`,
        room.engine.connectedServerAddress,
      );

      initRoom();
    } catch (error: any) {
      let message: any = error;
      if (error.message) {
        message = error.message;
      }
      appendLog('could not connect:', message);
      const btnJoin = <HTMLButtonElement>$("btnJoin");
      btnJoin.innerHTML = "JOIN";
      btnJoin.removeAttribute('disabled');

      Swal.fire({
        title: 'Error',
        text: 'Could not connect. Please try again',
        showCancelButton: false,
        confirmButtonColor: '#0E78F9',
        cancelButtonColor: '#292C3D',
        confirmButtonText: 'Ok',
        showCloseButton: true,
        heightAuto: false,
        background: '#292c3d',
      }).then(() => {
        window.location.reload();
      });

      return;
    }
    currentRoom = room;
    window.currentRoom = room;
    setButtonsForState(true);
    updateButtonsForPublishState();

    room.participants.forEach((participant) => {
      participantConnected(participant);
    });
    participantConnected(room.localParticipant);

    return room;
  },

  toggleAudio: async () => {
    if (!currentRoom) return;
    const enabled = currentRoom.localParticipant.isMicrophoneEnabled;
    setButtonDisabled('toggle-audio-button', true);
    if (enabled) {
      appendLog('disabling audio');
    } else {
      appendLog('enabling audio');
    }
    await currentRoom.localParticipant.setMicrophoneEnabled(!enabled);
    setButtonDisabled('toggle-audio-button', false);
    updateButtonsForPublishState();
    acquireDeviceList("audio-input");
    acquireDeviceList("audio-output");
  },

  toggleVideo: async () => {
    if (!currentRoom) return;
    setButtonDisabled('toggle-video-button', true);
    const enabled = currentRoom.localParticipant.isCameraEnabled;
    if (enabled) {
      appendLog('disabling video');
    } else {
      appendLog('enabling video');
    }
    await currentRoom.localParticipant.setCameraEnabled(!enabled);
    setButtonDisabled('toggle-video-button', false);
    renderParticipant(currentRoom.localParticipant);

    // update display
    updateButtonsForPublishState();
    acquireDeviceList("video-output");
  },

  flipVideo: () => {
    const videoPub = currentRoom?.localParticipant.getTrack(Track.Source.Camera);
    if (!videoPub) {
      return;
    }
    if (state.isFrontFacing) {
      setButtonState('flip-video-button', 'Front Camera', false);
    } else {
      setButtonState('flip-video-button', 'Back Camera', false);
    }
    state.isFrontFacing = !state.isFrontFacing;
    const options: VideoCaptureOptions = {
      resolution: VideoPresets.h720.resolution,
      facingMode: state.isFrontFacing ? 'user' : 'environment',
    };
    videoPub.videoTrack?.restartTrack(options);
  },

  shareScreen: async () => {
    if (!currentRoom) return;

    const enabled = currentRoom.localParticipant.isScreenShareEnabled;
    appendLog(`${enabled ? 'stopping' : 'starting'} screen share`);
    setButtonDisabled('share-screen-button', true);
    try {
      await currentRoom.localParticipant.setScreenShareEnabled(!enabled, { audio: true });
    } catch (error) {
      console.log("share screen error: ", error);
      if (error instanceof Error && error.message != "Permission denied") {
        Swal.fire({
          title: 'Warning',
          text: `${error}`,
          showCancelButton: false,
          confirmButtonColor: '#0E78F9',
          cancelButtonColor: '#292C3D',
          confirmButtonText: 'Ok',
          showCloseButton: true,
          heightAuto: false,
          background: '#292c3d',
        });
      }
    }
    setButtonDisabled('share-screen-button', false);
    updateButtonsForPublishState();

    if (enabled) {
      (<HTMLDivElement>$(`screenshare-wrapper-${currentRoom.localParticipant.identity}`))?.remove();
      handleLayouts();
    }
  },

  startAudio: () => {
    currentRoom?.startAudio();
  },

  enterText: () => {
    if (!currentRoom) return;
    const textField = <HTMLInputElement>$('entry');
    if (textField.value) {
      const msg = state.encoder.encode(textField.value);
      currentRoom.localParticipant.publishData(msg, DataPacket_Kind.RELIABLE);
      const dateNow = new Date;
      let text = textField.value;
      if (/[(http(s)?):\/\/(www\.)?a-zA-Z0-9@:%._\+~#=]{2,256}\.[a-z]{2,6}\b([-a-zA-Z0-9@:%_\+.~#?&//=]*)/.test(text)) {
        text = `<a href="${text}" target="_blank">${text}<svg width="14" height="14"><use xlink:href="#ExternalLink"></use></svg></a>`;
      }
      
      const chatTemplate = `
      <div class="box-chat__inner wrap-mess reply">
        <div class="box-chat__inner head">
          <span class="time">${dateNow.getHours()}:${dateNow.getMinutes()}</span>
        </div>
        <div class="box-chat__inner content">
          <div class="box-messenger">
            <div class="messenger">${text}</div>
          </div>
          <span class="user-seen">
            <i class="icon-checkcircle"></i>
          </span>
        </div>
      </div>`;

      const chatMessageElm = <HTMLDivElement>$('chat-message');
      chatMessageElm.insertAdjacentHTML("beforeend", chatTemplate);
      textField.value = '';
      if (chatMessageElm.parentElement) {
        chatMessageElm.parentElement.scrollTop = chatMessageElm.scrollHeight;
      }
    }

    return false;
  },

  disconnectRoom: () => {
    if (currentRoom) {
      currentRoom.disconnect();
    }
    if (state.bitrateInterval) {
      clearInterval(state.bitrateInterval);
    }
  },

  handleScenario: (e: Event) => {
    const scenario = (<HTMLSelectElement>e.target).value;
    if (scenario !== '') {
      currentRoom?.simulateScenario(scenario);
      (<HTMLSelectElement>e.target).value = '';
    }
  },

  handleDeviceSelected: async (e: Event) => {
    const deviceId = (<HTMLSelectElement>e.target).value;
    const elementId = (<HTMLSelectElement>e.target).id;
    const kind = elementMapping[elementId];
    if (!kind) {
      return;
    }

    state.defaultDevices.set(kind, deviceId);

    if (currentRoom) {
      await currentRoom.switchActiveDevice(kind, deviceId);
    }
  },

  handlePreferredQuality: (e: Event) => {
    const quality = (<HTMLSelectElement>e.target).value;
    let q = VideoQuality.HIGH;
    switch (quality) {
      case 'low':
        q = VideoQuality.LOW;
        break;
      case 'medium':
        q = VideoQuality.MEDIUM;
        break;
      case 'high':
        q = VideoQuality.HIGH;
        break;
      default:
        break;
    }
    if (currentRoom) {
      currentRoom.participants.forEach((participant) => {
        participant.tracks.forEach((track) => {
          track.setVideoQuality(q);
        });
      });
    }
  },

  hideRoomButtons: () => {
    const settingsBtn = <HTMLButtonElement>$("settings-btn");
    settingsBtn.classList.remove("active");
  },

  toggleChat: () => {
    const chatBtn = <HTMLButtonElement>$("chat-btn");
    chatBtn.classList.toggle("active");
    chatBtn.classList.remove("notify");
    
    (<HTMLDivElement>document.querySelector(".wrap-box-frame__inner")).classList.toggle("show-chat");
    (<HTMLDivElement>$("box-chat")).classList.toggle("active");
  },

  copyMeetingLink: async () => {
    if (navigator.clipboard) {
      navigator.clipboard.writeText(location.href).then(function() {
        console.log('Async: Copying to clipboard was successful!');
      }, function(err) {
        console.error('Async: Could not copy text: ', err);
      });

      // copyImageToClipboard(`${location.origin}/qr-code${location.pathname}`)
      // .then(() => {
      //   console.log('Image Copied')
      // })
      // .catch((e) => {
      //   console.log('Error: ', e.message)
      // });

      // const fetchedImageData = await fetch(`${location.origin}/qr-code${location.pathname}`);
      // const blob = await fetchedImageData.blob();
      // await navigator.clipboard.write([
      //     new ClipboardItem({
      //         [blob.type]: blob
      //     }),
      //     new ClipboardItem({
      //       ['text/plain']: location.href
      //   })
      // ]).then(function() {
      //   console.log('Async: Copying to clipboard was successful!');
      // }, function(err) {
      //   console.error('Async: Could not copy text: ', err);
      // });
    } else {
      let textArea = <HTMLTextAreaElement>document.createElement("textarea");
      textArea.style.position = 'fixed';
      textArea.style.top = "0";
      textArea.style.left = "0";
      textArea.style.width = '2em';
      textArea.style.height = '2em';
      textArea.style.padding = "0";
      textArea.style.border = 'none';
      textArea.style.outline = 'none';
      textArea.style.boxShadow = 'none';
      textArea.style.background = 'transparent';
      textArea.value = location.href;
      document.body.appendChild(textArea);
      textArea.focus();
      textArea.select();
      try {
        let successful = document.execCommand('copy');
        let msg = successful ? 'successful' : 'unsuccessful';
        console.log('Copying text command was ' + msg);
      } catch (err) {
        console.log('Oops, unable to copy');
      }
      document.body.removeChild(textArea);
    }

    const Toast = Swal.mixin({
      toast: true,
      position: 'top-end',
      showConfirmButton: false,
      timer: 3000,
      timerProgressBar: true,
      background: '#292c3d',
      didOpen: (toast: any) => {
        toast.addEventListener('mouseenter', Swal.stopTimer)
        toast.addEventListener('mouseleave', Swal.resumeTimer)
      }
    });
    Toast.fire({
      icon: 'success',
      title: 'Copied meeting link',
      color: '#fff'
    });
  },

  toggleFullscreen: () => {
    if (fscreen.fullscreenEnabled) {
      const btnFullscreen = <HTMLAnchorElement>$("btnFullscreen");
      if (fscreen.fullscreenElement !== null) {
        btnFullscreen.innerHTML = '<i class="icon-Maximize"></i>';
        fscreen.exitFullscreen();
      } else {
        btnFullscreen.innerHTML = '<i class="icon-Minimize"></i>';
        fscreen.requestFullscreen(document.documentElement);
      }
    }
  },

  raiseHand: () => {
    let code;
    if (state.raiseHand) {
      state.raiseHand = false;
      code = ":lowerhand:";
    } else {
      state.raiseHand = true;
      code = ":raisehand:";
    }
    const msg = state.encoder.encode(code);
    currentRoom?.localParticipant.publishData(msg, DataPacket_Kind.RELIABLE);
    if (currentRoom?.localParticipant) {
      raiseHand(currentRoom?.localParticipant, !state.raiseHand);
    }
    const raiseHandButtonText = <HTMLSpanElement>document.querySelector("#raise-hand-button span");
    if (raiseHandButtonText) {
      raiseHandButtonText.innerHTML = state.raiseHand ? "Lower hand" : "Raise hand";
    }
  },
};

const btnSettings = <HTMLDivElement>$("settings-btn");
btnSettings.addEventListener("click", (e: Event) => {
  if (e.target == btnSettings.querySelector(".icon-Setting") || e.target == btnSettings) {
    btnSettings.classList.toggle("active");
  }
});

declare global {
  interface Window {
    currentRoom: any;
    appActions: typeof appActions;
    process: any;
  } 
}

window.appActions = appActions;

declare const SOUNDS: any;

// --------------------------- event handlers ------------------------------- //

function initRoom() {
  // remove join page
  (<HTMLDivElement>$("joinPage")).remove();
  let roomPartition = document.querySelectorAll(".room-partition");
  if (roomPartition.length > 0) {
    roomPartition.forEach((el) => {
      (<HTMLDivElement>el).classList.remove("hide")
    });
  }
  setTimeout(() => {
    readyRecieveNotify = true;
  }, 2000);

  // fullscreen
  fscreen.addEventListener('fullscreenchange', () => {
    const btnFullscreen = <HTMLAnchorElement>$("btnFullscreen");
    if (fscreen.fullscreenElement !== null) {
      btnFullscreen.innerHTML = '<i class="icon-Minimize"></i>';
    } else {
      btnFullscreen.innerHTML = '<i class="icon-Maximize"></i>';
    }
  }, false);
  document.addEventListener('keydown', (event: any) => {
    if (event.which === 122 || event.key === "F11") {
      event.preventDefault();
      appActions.toggleFullscreen();
    }
  });
}

function notify(message: string, type: string | undefined = undefined) {
  if (!readyRecieveNotify) return;

  let audioFile = type ? SOUNDS[type] : SOUNDS["notify"];
  let sound = new Audio(audioFile);
  sound.volume = 0.2;
  sound.play();

  let notifyPopup = Swal.mixin({
    toast: true,
    position: 'bottom-left',
    showConfirmButton: false,
    timer: 2000,
    timerProgressBar: true,
    background: '#292c3d',
    didOpen: (toast: any) => {
      toast.addEventListener('mouseenter', Swal.stopTimer)
      toast.addEventListener('mouseleave', Swal.resumeTimer)
    }
  });
  notifyPopup.fire({
    title: message,
    color: '#fff',
  });
}

function handleLayoutItems() {
  const items = document.querySelectorAll(".box-frame .box-video-call");
  if (items.length == 1 || items.length >= 4) {
    document.querySelector(".box-frame")?.classList.add("full-frame");
  } else {
    document.querySelector(".box-frame")?.classList.remove("full-frame");
  }
}

function raiseHand(participant: Participant, isLower: boolean | undefined = undefined) {
  if (!isLower) {
    let sound = new Audio(SOUNDS["notify"]);
    sound.volume = 0.2;
    sound.play();

    let raiseHandPopup = Swal.mixin({
      toast: true,
      position: 'top-right',
      showConfirmButton: false,
      timer: 5000,
      timerProgressBar: true,
      background: '#292c3d',
      didOpen: (toast: any) => {
        toast.addEventListener('mouseenter', Swal.stopTimer)
        toast.addEventListener('mouseleave', Swal.resumeTimer)
      }
    });
    raiseHandPopup.fire({
      title: `<svg width="30" height="30"><use xlink:href="#RaiseHand"></use></svg>${participant.name} raise hand`,
      color: '#fff',
    });
  }

  const icon = <HTMLElement>$(`raise-hand-${participant.identity}`);
  if (icon) {
    if (isLower) {
      icon.classList.add("hide");
    } else {
      icon.classList.remove("hide");
    }
  }
}

function handleData(msg: Uint8Array, participant?: RemoteParticipant) {
  if (!participant) return;

  const str = state.decoder.decode(msg);
  if (/^:raisehand:$/g.test(str)) {
    raiseHand(participant);
    return;
  } else if (/^:lowerhand:$/g.test(str)) {
    raiseHand(participant, true);
    return;
  }

  const chatBtn = <HTMLButtonElement>$("chat-btn");
  if (!chatBtn.classList.contains("active") && !chatBtn.classList.contains("notify")) {
    chatBtn.classList.add("notify");
    notify(`${participant.name} <br> ${str}`, "message");
  }
  
  const dateNow = new Date;
  const avatar = getAvatar(participant);
  const chatMessageElm = <HTMLDivElement>$('chat-message');

  let text = str;
  if (/[(http(s)?):\/\/(www\.)?a-zA-Z0-9@:%._\+~#=]{2,256}\.[a-z]{2,6}\b([-a-zA-Z0-9@:%_\+.~#?&//=]*)/.test(text)) {
    text = `<a href="${text}" target="_blank">${text}<svg width="14" height="14"><use xlink:href="#ExternalLink"></use></svg></a>`;
  }

  const chatTemplate = `
  <div class="box-chat__inner wrap-mess">
    <div class="box-chat__inner head">
      <span class="nickname">${participant.name}</span>
      <span class="time">${dateNow.getHours()}:${dateNow.getMinutes()}</span>
    </div>
    <div class="box-chat__inner content">
      <span class="avatar">
        <div class="box-info-user">
          <div class="box-avatar ${!avatar ? "default" : ""}">
            <i class="icon-Profile"></i>
            ${avatar ? "<img src="+avatar+">" : ""}
          </div>
        </div>
      </span>
      <div class="box-messenger">
        <div class="messenger">${text}</div>
      </div>
    </div>
  </div>
  `;
  chatMessageElm.insertAdjacentHTML("beforeend", chatTemplate);
  if (chatMessageElm.parentElement) {
    chatMessageElm.parentElement.scrollTop = chatMessageElm.scrollHeight;
  }
}

function updateRoomInfo() {
  let num = 1;
  if (currentRoom?.participants) {
    num = currentRoom?.participants.size + 1;
  }
  (<HTMLSpanElement>$("participants-counter")).innerHTML = num + " participants";

  if (currentRoom?.name) {
    let roomName = currentRoom?.name;
    if (currentRoom.metadata && currentRoom.metadata != "") {
      let metaData = JSON.parse(currentRoom.metadata);
      if (metaData.real_name && metaData.real_name != "") {
        roomName = metaData.real_name;
      }
    }
    (<HTMLDivElement>$("room-name")).innerHTML = roomName;
  }
}

function participantConnected(participant: Participant) {
  appendLog('participant', participant.name, 'connected', participant.metadata);
  updateRoomInfo();
  if (!participant.isCameraEnabled && !participant.isMicrophoneEnabled) {
    appendLog('connected participant empty track', participant.name);
    renderParticipant(participant);
    if (currentRoom) {
      renderScreenShare(currentRoom);
    }
  }
  participant
    .on(ParticipantEvent.TrackMuted, (pub: TrackPublication) => {
      appendLog('track was muted', pub.trackSid, participant.name);
      renderParticipant(participant);
    })
    .on(ParticipantEvent.TrackUnmuted, (pub: TrackPublication) => {
      appendLog('track was unmuted', pub.trackSid, participant.name);
      renderParticipant(participant);
    })
    .on(ParticipantEvent.IsSpeakingChanged, () => {
      renderParticipant(participant);
    })
    .on(ParticipantEvent.ConnectionQualityChanged, () => {
      renderParticipant(participant);
    });
}

function participantDisconnected(participant: RemoteParticipant) {
  appendLog('participant', participant.sid, 'disconnected');
  
  updateRoomInfo()
  renderParticipant(participant, true);

  let screenShare = <HTMLDivElement>$(`screenshare-wrapper-${participant.identity}`);
  if (screenShare) {
    screenShare.remove();
    handleLayouts();
  }
}

function handleRoomDisconnect(reason?: DisconnectReason) {
  if (!currentRoom) return;
  appendLog('disconnected from room', { reason });
  setButtonsForState(false);
  // renderParticipant(currentRoom.localParticipant, true);
  // currentRoom.participants.forEach((p) => {
  //   renderParticipant(p, true);
  // });
  // renderScreenShare(currentRoom);
  (<HTMLDivElement>document.querySelector(".page-video-call")).innerHTML = '<div class="container"><p style="text-align: center">Leaving the meeting...</p></div>';

  // currentRoom = undefined;
  // window.currentRoom = undefined;
  window.location.href = window.location.href;
}

// -------------------------- rendering helpers ----------------------------- //

function appendLog(...args: any[]) {
  for (let i = 0; i < arguments.length; i += 1) {
    if (typeof args[i] === 'object') {
      console.log("LOG: ", `${JSON && JSON.stringify ? JSON.stringify(args[i], undefined, 2) : args[i]
        } `);
    } else {
      console.log("LOG: ", `${args[i]} `);
    }
  }
}

function getAvatar(participant: Participant): string {
  let metadata = participant.metadata ? JSON.parse(participant.metadata) : "";
  let avatar;
  if (metadata.avatar && metadata.avatar != 'null') {
    avatar = metadata.avatar;
  }
  return avatar
}

// updates participant UI
function renderParticipant(participant: Participant, remove: boolean = false) {
  let container = <HTMLDivElement>$(`participants-area${(state.layout == 2) ? "-2" : "" }`);
  if (state.layout == 2 && container.hasAttribute("data-simplebar")) {
    container = <HTMLDivElement>container.querySelector(".simplebar-content");
  }

  if (!container) return;
  const { identity, name } = participant;

  let div = $(`participant-${identity}`);
  if (!div && !remove) {
    div = document.createElement('div');
    div.id = `participant-${identity}`;
    div.classList.add("box-video-call");

    let isHost = false;
    if (currentRoom && currentRoom.metadata && currentRoom.metadata != "") {
      let metaData = JSON.parse(currentRoom.metadata);
      if (metaData.host && metaData.host != "" && identity == metaData.host) {
        isHost = true;
      }
    }

    let avatar = getAvatar(participant);
    div.innerHTML = `
    <div class="box-video-call__inner">
      <a href="javascript:void(0);" class="raise-hand hide" id="raise-hand-${identity}">
        <svg width="25" height="25"><use xlink:href="#RaiseHand"></use></svg>
      </a>
      <div class="box-info-user" id="box-avatar-${identity}">
        <div class="box-avatar ${!avatar ? "default" : ""}">
          <i class="icon-Profile"></i>
          ${avatar ? "<img src="+avatar+">" : ""}
        </div>
      </div>
      <div class="box-video hide" id="box-video-${identity}">
        <video id="video-${identity}"></video>
        <audio id="audio-${identity}"></audio>
        <div class="info-bar hide">
          <div style="text-align: center;">
            <span id="codec-${identity}" class="codec"></span>
            <span id="size-${identity}" class="size"></span>
            <span id="bitrate-${identity}" class="bitrate"></span>
          </div>
        </div>
      </div>
      <span class="name" id="name-${identity}">${name}</span>
      <div class="group-button">
        ${isHost ? `<span class="p-host">host</span>`: ""}
        <a href="javascript:void(0);" class="item ic-voice hide" id="mic-${identity}">
          <i class="icon-Voiceoff"></i>
        </a>
      </div>
      <div class="box-voice hide" id="speaking-${identity}">
        <div class="box-voice__inner"><div></div><div></div><div></div><div></div><div></div></div>
      </div>
    </div>
    ${participant instanceof RemoteParticipant ?
      `<div class="volume-control hide">
      <input id="volume-${identity}" type="range" min="0" max="1" step="0.1" value="1" orient="vertical" />
    </div>` : ''}`;
    container.appendChild(div);

    const sizeElm = $(`size-${identity}`);
    const videoElm = <HTMLVideoElement>$(`video-${identity}`);
    videoElm.onresize = () => {
      updateVideoSize(videoElm!, sizeElm!);
    };

    handleLayoutItems();
    if (participant instanceof RemoteParticipant) {
      notify(`${name} joined`, "join-room");
    }
  }
  const videoElm = <HTMLVideoElement>$(`video-${identity}`);
  const audioELm = <HTMLAudioElement>$(`audio-${identity}`);
  if (remove) {
    div?.remove();
    if (videoElm) {
      videoElm.srcObject = null;
      videoElm.src = '';
    }
    if (audioELm) {
      audioELm.srcObject = null;
      audioELm.src = '';
    }
    handleLayoutItems();
    if (participant instanceof RemoteParticipant) {
      notify(`${name} has left the meeting`, "left-room");
    }
    return;
  }

  // update properties
  const nameElm = <HTMLSpanElement>$(`name-${identity}`);
  const micElm = $(`mic-${identity}`)!;
  const speakingElm = $(`speaking-${identity}`)!;
  const cameraPub = participant.getTrack(Track.Source.Camera);
  const micPub = participant.getTrack(Track.Source.Microphone);
  const participantElm = $(`participant-${identity}`);

  nameElm.innerHTML = name ?? "";
  if (participant instanceof LocalParticipant) {
    nameElm.innerHTML += ' (you)';
  }
  if (participant.isSpeaking) {
    speakingElm.classList.remove("hide");
    participantElm?.classList.add("speaking");

    if (div!.offsetTop - container.offsetTop >= div!.offsetHeight) {
      container.insertAdjacentElement("afterbegin", div!);
    }
  } else {
    speakingElm.classList.add("hide");
    participantElm?.classList.remove("speaking");
  }

  if (participant instanceof RemoteParticipant) {
    const volumeSlider = <HTMLInputElement>$(`volume-${identity}`);
    volumeSlider.addEventListener('input', (ev) => {
      participant.setVolume(Number.parseFloat((ev.target as HTMLInputElement).value));
    });
  }

  const cameraEnabled = cameraPub && cameraPub.isSubscribed && !cameraPub.isMuted;
  const boxAvatar = <HTMLDivElement>$(`box-avatar-${identity}`);
  const boxVideo = <HTMLDivElement>$(`box-video-${identity}`);
  if (cameraEnabled) {
    boxAvatar.classList.add("hide");
    boxVideo.classList.remove("hide");
    if (participant instanceof LocalParticipant) {
      // flip
      // videoElm.style.transform = 'scale(-1, 1)';
    } else if (!cameraPub?.videoTrack?.attachedElements.includes(videoElm)) {
      const renderStartTime = Date.now();
      // measure time to render
      videoElm.onloadeddata = () => {
        const elapsed = Date.now() - renderStartTime;
        let fromJoin = 0;
        if (participant.joinedAt && participant.joinedAt.getTime() < startTime) {
          fromJoin = Date.now() - startTime;
        }
        appendLog(
          `RemoteVideoTrack ${cameraPub?.trackSid} (${videoElm.videoWidth}x${videoElm.videoHeight}) rendered in ${elapsed}ms`,
          fromJoin > 0 ? `, ${fromJoin}ms from start` : '',
        );
      };
    }
    cameraPub?.videoTrack?.attach(videoElm);
  } else {
    boxVideo.classList.add("hide");
    boxAvatar.classList.remove("hide");
    // clear information display
    $(`size-${identity}`)!.innerHTML = '';
    if (cameraPub?.videoTrack) {
      // detach manually whenever possible
      cameraPub.videoTrack?.detach(videoElm);
    } else {
      videoElm.src = '';
      videoElm.srcObject = null;
    }
  }

  const micEnabled = micPub && micPub.isSubscribed && !micPub.isMuted;
  if (micEnabled) {
    if (participant instanceof LocalParticipant) {
      (<HTMLButtonElement>$("toggle-audio-button")).innerHTML = '<i class="icon-Voice"></i><span class="tooltiptext">Turn off Mic</span>';
    } else {
      // don't attach local audio
      audioELm.onloadeddata = () => {
        if (participant.joinedAt && participant.joinedAt.getTime() < startTime) {
          const fromJoin = Date.now() - startTime;
          appendLog(`RemoteAudioTrack ${micPub?.trackSid} played ${fromJoin}ms from start`);
        }
      };
      micPub?.audioTrack?.attach(audioELm);
    }
    micElm.classList.add("hide");
  } else {
    if (participant instanceof LocalParticipant) {
      (<HTMLButtonElement>$("toggle-audio-button")).innerHTML = '<i class="icon-Voiceoff"></i><span class="tooltiptext">Turn on Mic</span>';
    }
    micElm.classList.remove("hide");
  }
}

function renderScreenShare(room: Room) {
  const div = <HTMLDivElement>$('screenshare-area');

  if (room.state !== ConnectionState.Connected) {
    state.layout = 1;
    div.classList.add("hide")
    return;
  }
  
  let screenSharePub = [];

  // get screen share from local
  let screenSharePubLocal: TrackPublication | undefined = room.localParticipant.getTrack(
    Track.Source.ScreenShare,
  );
  let screenShareAudioPubLocal: RemoteTrackPublication | undefined;
  if(screenSharePubLocal) {
    screenSharePub.push({track: screenSharePubLocal, participant: room.localParticipant})
  }

  // get screen share from participants
  room.participants.forEach((p) => {
    const pub = p.getTrack(Track.Source.ScreenShare);
    if (pub?.isSubscribed) {
      screenSharePub.push({track: pub, participant: p})
    }
    const audioPub = p.getTrack(Track.Source.ScreenShareAudio);
    if (audioPub?.isSubscribed) {
      screenShareAudioPubLocal = audioPub;
    }
  });
  
  if (screenSharePub.length > 0) {
    state.layout = 2;
    div.classList.remove("hide");

    screenSharePub.forEach((screenShareInfo) => {
      let screenshare = <HTMLDivElement>$(`screenshare-wrapper-${screenShareInfo.participant.identity}`);
      if (!screenshare) {
        div.insertAdjacentHTML("beforeend", `
        <div class="box-video__outer" id="screenshare-wrapper-${screenShareInfo.participant.identity}">
          <div class="box-video__inner">
            <video poster="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7" id="screenshare-video-${screenShareInfo.participant.identity}" autoplay playsinline${isMobile ? " controls" : ""} class="hide-controls-timeline hide-play-button hide-volume-slider hide-mute-button"></video>
          </div>
          ${!isMobile ? `
          <a href="javascript:void(0);" class="ic-maximize" id="screenshare-fullscreen-${screenShareInfo.participant.identity}" style="opacity:0">
            <i class="icon-Maximize" style="background-color: rgba(28,31,45,.7);"></i>
          </a>
          ` : ""}
          <div class="user-share-tooltip">Screenshare from ${screenShareInfo.participant.name} <span id="screenshare-resolution-${screenShareInfo.participant.identity}"> </span></div>
        </div>
        `);

        handleLayouts();

        const videoElm = $(`screenshare-video-${screenShareInfo.participant.identity}`) as HTMLVideoElement;
        screenShareInfo.track.videoTrack?.attach(videoElm);
        if (screenShareAudioPubLocal) {
          screenShareAudioPubLocal.audioTrack?.attach(videoElm);
        }
        videoElm.onresize = () => {
          updateVideoSize(videoElm, <HTMLSpanElement>$(`screenshare-resolution-${screenShareInfo.participant.identity}`));
        };
        videoElm.addEventListener("click", (e: Event) => {
          e.preventDefault();
          return false;
        });

        if (!isMobile) {
          const fullScreenBtn = <HTMLAnchorElement>$(`screenshare-fullscreen-${screenShareInfo.participant.identity}`);
          fullScreenBtn.addEventListener("click", () => {
            fscreen.requestFullscreen(videoElm);
          });
        }
      }
    });
  } else {
    state.layout = 1;
    div.classList.add("hide");
  }
}

function handleLayouts() {
  const screenShareArea = <HTMLDivElement>$("screenshare-area");
  const pageVideoCall = <HTMLElement>document.querySelector(".page-video-call");
  const layout1 = <HTMLDivElement>$("layout-1");
  const layout2 = <HTMLDivElement>$("layout-2");
  if (screenShareArea?.childElementCount > 0) {
    state.layout = 2;
    pageVideoCall.classList.add("list-frame");
    layout1.classList.add("hide");
    layout2.classList.remove("hide");

    if (screenShareArea?.childElementCount >= 2) {
      screenShareArea.classList.add("multi-screen");
    } else {
      screenShareArea.classList.remove("multi-screen");
    }
  } else {
    state.layout = 1;
    pageVideoCall.classList.remove("list-frame");
    layout2.classList.add("hide");
    layout1.classList.remove("hide");
  }
  
  if (state.layout == 2) {
    let participantsArea = $("participants-area");
    if (participantsArea && participantsArea?.childElementCount > 0) {
      let participantsArea2 = $("participants-area-2");
      if (participantsArea2) {
        if (participantsArea2.hasAttribute("data-simplebar")) {
          participantsArea2?.querySelector(".simplebar-content")?.append(...participantsArea.childNodes);
        } else {
          participantsArea2.append(...participantsArea.childNodes);
          if (isMobile) {
            layout2.querySelector(".scrollbar-outer")?.classList.remove("scrollbar-outer");
          } else {
            new SimpleBar(participantsArea2);
          }
        }
      }
    }

    let boxChat = $("box-chat");
    if (boxChat?.parentElement?.id == "layout-1") {
      layout2.append(boxChat);
    }
  } else {
    let participantsArea2 = isMobile ? $("participants-area-2") : $("participants-area-2")?.querySelector(".simplebar-content");
    if (participantsArea2 && participantsArea2?.childElementCount > 0) {
      let participantsArea = $("participants-area");
      if (participantsArea) {
        participantsArea.append(...participantsArea2.childNodes);
        handleLayoutItems();
      }
    }

    let boxChat = $("box-chat");
    if (boxChat?.parentElement?.id == "layout-2") {
      layout1.append(boxChat);
    }
  }
}

function renderBitrate() {
  if (!currentRoom || currentRoom.state !== ConnectionState.Connected) {
    return;
  }
  const participants: Participant[] = [...currentRoom.participants.values()];
  participants.push(currentRoom.localParticipant);

  for (const p of participants) {
    const elm = $(`bitrate-${p.identity}`);
    let totalBitrate = 0;
    for (const t of p.tracks.values()) {
      if (t.track) {
        totalBitrate += t.track.currentBitrate;
      }

      if (t.source === Track.Source.Camera) {
        if (t.videoTrack instanceof RemoteVideoTrack) {
          const codecElm = $(`codec-${p.identity}`)!;
          codecElm.innerHTML = t.videoTrack.getDecoderImplementation() ?? '';
        }
      }
    }
    let displayText = '';
    if (totalBitrate > 0) {
      displayText = `${Math.round(totalBitrate / 1024).toLocaleString()} kbps`;
    }
    if (elm) {
      elm.innerHTML = displayText;
    }
  }
}

function updateVideoSize(element: HTMLVideoElement, target: HTMLElement) {
  if (element && target) {
    target.innerHTML = `(${element.videoWidth}x${element.videoHeight})`;
  }
}

function setButtonState(
  buttonId: string,
  buttonText: string,
  isActive: boolean,
  isDisabled: boolean | undefined = undefined,
) {
  const el = $(buttonId) as HTMLButtonElement;
  if (!el) return;
  if (isDisabled !== undefined) {
    el.disabled = isDisabled;
  }
  el.innerHTML = buttonText;
  if (isActive) {
    el.classList.add('active');
  } else {
    el.classList.remove('active');
  }
}

function setButtonDisabled(buttonId: string, isDisabled: boolean) {
  const el = $(buttonId) as HTMLButtonElement;
  el.disabled = isDisabled;
}

function setButtonsForState(connected: boolean) {
  const connectedSet = [
    'toggle-video-button',
    'toggle-audio-button',
    'share-screen-button',
    'disconnect-ws-button',
    'disconnect-room-button',
    'flip-video-button',
    'send-button',
  ];
  const disconnectedSet = ['connect-button'];

  const toRemove = connected ? connectedSet : disconnectedSet;
  const toAdd = connected ? disconnectedSet : connectedSet;

  toRemove.forEach((id) => $(id)?.removeAttribute('disabled'));
  toAdd.forEach((id) => $(id)?.setAttribute('disabled', 'true'));
}

const elementMapping: { [k: string]: MediaDeviceKind } = {
  'video-input': 'videoinput',
  'audio-input': 'audioinput',
  'audio-output': 'audiooutput',
};
async function handleDevicesChanged() {
  Promise.all(
    Object.keys(elementMapping).map(async (id) => {
      const kind = elementMapping[id];
      if (!kind) {
        return;
      }
      const devices = await Room.getLocalDevices(kind);
      console.log(id);
      const element = <HTMLSelectElement>$(id);
      populateSelect(element, devices, state.defaultDevices.get(kind));
    }),
  );
}

function populateSelect(
  element: HTMLSelectElement,
  devices: MediaDeviceInfo[],
  selectedDeviceId?: string,
) {
  // clear all elements
  element.innerHTML = '';

  for (const device of devices) {
    const option = document.createElement('option');
    option.text = device.label;
    option.value = device.deviceId;
    if (device.deviceId === selectedDeviceId) {
      option.selected = true;
    }
    element.appendChild(option);
  }
}

function updateButtonsForPublishState() {
  if (!currentRoom) {
    return;
  }
  const lp = currentRoom.localParticipant;

  // video
  setButtonState(
    'toggle-video-button',
    `${lp.isCameraEnabled ? '<i class="icon-Video"></i><span class="tooltiptext">Turn off Camera</span>' : '<i class="icon-Videooff"></i><span class="tooltiptext">Turn on Camera</span>'}`,
    false,
  );

  // audio
  setButtonState(
    'toggle-audio-button',
    `${lp.isMicrophoneEnabled ? '<i class="icon-Voice"></i><span class="tooltiptext">Turn off Mic</span>' : '<i class="icon-Voiceoff"></i><span class="tooltiptext">Turn on Mic</span>'}`,
    false,
  );

  // screen
  const shareScreenBtn = <HTMLButtonElement>$("share-screen-button");
  if (lp.isScreenShareEnabled) {
    shareScreenBtn.classList.add("active");
    shareScreenBtn.innerHTML = '<i class="icon-sharescreen"></i><span class="tooltiptext">Stop presenting</span>';
  } else {
    shareScreenBtn.classList.remove("active");
    shareScreenBtn.innerHTML = '<i class="icon-sharescreen"></i><span class="tooltiptext">Present now</span>';
  }
}

async function acquireDeviceList(id: string) {
  const optsNum = $(id)?.childElementCount;
  if (optsNum && optsNum <= 1) {
    const kind = elementMapping[id];
    if (!kind) {
      return;
    }
    const devices = await Room.getLocalDevices(kind);
    console.log(id);
    const element = <HTMLSelectElement>$(id);
    populateSelect(element, devices, state.defaultDevices.get(kind));
  }
}

let timerInterval: any;
function startCountTimer() {
  if (timerInterval) clearInterval(timerInterval);
  const timer = <HTMLSpanElement>$("room-duration");
  let startTime = parseInt(timer.getAttribute("data-start") ?? "0");
  let totalSeconds = 0;
  if (startTime > 0) {
    totalSeconds = Math.floor((new Date).getTime()/1000) - startTime;
    if (totalSeconds < 0) {
      totalSeconds = 0;
    }
  }
  timerInterval = setInterval(() => {
    ++totalSeconds;
    let hour = Math.floor(totalSeconds /3600);
    let minute = Math.floor((totalSeconds - hour*3600)/60);
    let seconds = totalSeconds - (hour*3600 + minute*60);
    let strH, strM, strS;
    if(hour < 10) strH = "0"+hour;
    else strH = hour;
    if(minute < 10) strM = "0"+minute;
    else strM = minute;
    if(seconds < 10) strS = "0"+seconds;
    else strS = seconds;
    timer.innerHTML = strH + ":" + strM + ":" + strS;
  }, 1000);
}