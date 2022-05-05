import {
  ConnectionQuality,
  DataPacket_Kind,
  LocalParticipant,
  MediaDeviceFailure,
  Participant,
  ParticipantEvent,
  RemoteParticipant,
  Room,
  RoomConnectOptions,
  RoomEvent,
  RoomOptions,
  RoomState,
  setLogLevel,
  Track,
  TrackPublication,
  VideoCaptureOptions,
  VideoPresets,
  VideoCodec,
} from '../src/index';
import { LogLevel } from '../src/logger';
import { Buffer } from 'buffer';

const API_URL = "https://video-call.vnexpress.net";
const WSS_URL = "wss://live.vnexpress.net";
const THUMB_DEFAULT = "/assets/images/unknown-user.jpg";

const $ = (id: string) => document.getElementById(id);

const state = {
  isFrontFacing: false,
  encoder: new TextEncoder(),
  decoder: new TextDecoder(),
  defaultDevices: new Map<MediaDeviceKind, string>(),
  bitrateInterval: undefined as any,
};
let currentRoom: Room | undefined;

const searchParams = new URLSearchParams(window.location.search);
const storedRoomName = searchParams.get('room') ?? '';
(<HTMLInputElement>$('room')).value = storedRoomName;

// join page
let joinMic = false;
let joinCam = false;
let joinMicButton = $("btnJoinMic");
let joinCamButton = $("btnJoinCam");
if (joinMicButton && joinCamButton) {
  joinMicButton.addEventListener("click", (e) => {
    if (joinMicButton && joinMicButton.innerHTML == '<i class="fas fa-microphone-slash text-danger"></i>') {
      joinMic = true;
      joinMicButton.innerHTML = '<i class="fas fa-microphone"></i>';
    } else {
      joinMic = false;
      if (joinMicButton) {
        joinMicButton.innerHTML = '<i class="fas fa-microphone-slash text-danger"></i>';
      }
    }
  });
  joinCamButton.addEventListener("click", (e) => {
    if (joinCamButton && joinCamButton.innerHTML == '<i class="fas fa-webcam"></i>') {
      joinCam = false;
      joinCamButton.innerHTML = '<i class="fas fa-webcam-slash text-danger"></i>';
    } else {
      joinCam = true;
      if (joinCamButton) {
        joinCamButton.innerHTML = '<i class="fas fa-webcam"></i>';
      }
    }
  });
}

// chat
let chatButton = <HTMLButtonElement>$("chat-btn");
let btnCloseBoxChat = <HTMLAnchorElement>document.querySelector(".box-chat .ic-close");
let boxChat = <HTMLDivElement>document.querySelector(".box-chat");
let chatForm = <HTMLFormElement>document.querySelector(".box-chat form");
chatButton.addEventListener("click", () => {
  let boxChat = <HTMLDivElement>document.querySelector(".box-chat");
  boxChat.style.display = "unset";
});
btnCloseBoxChat.addEventListener("click", () => {
  boxChat.style.display = "none";
});
chatForm.addEventListener("submit", (e) => {
  e.preventDefault();
  return false;
});

// settings
let settingsButton = document.getElementById("settings-btn");
if (settingsButton) {
  let settingsModal = new bootstrap.Modal(document.querySelector('#settingsModal'));
  settingsButton.addEventListener("click", () => {
    settingsModal.toggle();
  });
}

function updateSearchParams(room: string) {
  const params = new URLSearchParams({ room });
  window.history.replaceState(null, '', `${window.location.pathname}?${params.toString()}`);
}

// handles actions from the HTML
const appActions = {
  connectWithFormInput: async () => {
    (<HTMLButtonElement>$("btnJoin")).innerHTML = "Joining...";
    let userName = (<HTMLInputElement>document.querySelector('#frm-join-meeting input[name="name"]')).value;
    let roomName = (<HTMLInputElement>document.querySelector('#frm-join-meeting input[name="room"]')).value;
    if (!roomName || !userName) {
      (<HTMLButtonElement>$("btnJoin")).innerHTML = "Join meeting";
      (<HTMLDivElement>document.querySelector('#notifyModal .modal-body')).innerHTML = "<p>Please provide your name and meeting id</p>";

      let notifyModal = new bootstrap.Modal(document.querySelector('#notifyModal'));
      notifyModal.show();

      return;
    }

    const token = await fetch(API_URL + "/api/get-join-token?user-name=" + userName + "&room-name=" + roomName).then(
      async (result) => {
        const {
          token
        } = await result.json();
        console.log("get-join-token", token);
        return token;
      }
    ).catch(() => {
      (<HTMLButtonElement>$("btnJoin")).innerHTML = "Join meeting";
    });

    const simulcast = (<HTMLInputElement>$('simulcast')).checked;
    const dynacast = (<HTMLInputElement>$('dynacast')).checked;
    const forceTURN = (<HTMLInputElement>$('force-turn')).checked;
    const adaptiveStream = (<HTMLInputElement>$('adaptive-stream')).checked;
    const publishOnly = (<HTMLInputElement>$('publish-only')).checked;
    const shouldPublish = (<HTMLInputElement>$('publish-option')).checked;
    const preferredCodec = (<HTMLSelectElement>$('preferred-codec')).value as VideoCodec;

    setLogLevel(LogLevel.debug);
    updateSearchParams(roomName);

    const roomOpts: RoomOptions = {
      adaptiveStream: adaptiveStream
        ? {
          pixelDensity: 'screen',
        }
        : false,
      dynacast,
      publishDefaults: {
        simulcast,
        videoSimulcastLayers: [VideoPresets.h90, VideoPresets.h216],
        videoCodec: preferredCodec,
      },
      videoCaptureDefaults: {
        resolution: VideoPresets.h720.resolution,
      },
    };

    const connectOpts: RoomConnectOptions = {
      autoSubscribe: !publishOnly,
      publishOnly: publishOnly ? 'publish_only' : undefined,
    };
    if (forceTURN) {
      connectOpts.rtcConfig = {
        iceTransportPolicy: 'relay',
      };
    }
    const room = await appActions.connectToRoom(WSS_URL, token, roomOpts, connectOpts);

    if (room && shouldPublish) {
      await Promise.all([
        room.localParticipant.setMicrophoneEnabled(joinMic),
        room.localParticipant.setCameraEnabled(joinCam),
      ]);
      updateButtonsForPublishState();
    }

    state.bitrateInterval = setInterval(renderBitrate, 1000);
  },

  connectToRoom: async (
    url: string,
    token: string,
    roomOptions?: RoomOptions,
    connectOptions?: RoomConnectOptions,
  ): Promise<Room | undefined> => {
    const room = new Room(roomOptions);
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
        renderScreenShare();
      })
      .on(RoomEvent.LocalTrackUnpublished, () => {
        renderParticipant(room.localParticipant);
        updateButtonsForPublishState();
        renderScreenShare();
      })
      .on(RoomEvent.DataReceived, (data) => {
        appendLog('new data received for room', data);

        const jsonString = Buffer.from(data).toString('utf8');
        const parsedData = JSON.parse(jsonString);
        
        if (parsedData) {
          if (typeof parsedData.toggle_camera == "boolean") {
            appActions.toggleVideo();
          }
          if (typeof parsedData.off_screenshare == "boolean") {
            appActions.shareScreen();
          }
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
          appendLog('connection quality changed', participant?.identity, quality);
        },
      );

    try {
      const start = Date.now();
      await room.connect(url, token, connectOptions);
      const elapsed = Date.now() - start;
      appendLog(
        `successfully connected to ${room.name} in ${Math.round(elapsed)}ms`,
        room.engine.connectedServerAddress,
      );

      // remove join page
      (<HTMLDivElement>$("joinPage")).remove();
      (<HTMLDivElement>$("meetingRoom")).style.display = "block";
    } catch (error) {
      let message: any = error;
      if ((<any>error).message) {
        message = (<any>error).message;
      }
      appendLog('could not connect:', message);
      (<HTMLButtonElement>$("btnJoin")).innerHTML = "Join meeting";
      return;
    }
    currentRoom = room;
    window.currentRoom = room;
    setButtonsForState(true);

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
      resolution: VideoPresets.qhd.resolution,
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
      await currentRoom.localParticipant.setScreenShareEnabled(!enabled);
    } catch (error) {
      console.log("share screen error: ", error);
      if (error.message != "Permission denied") {
        (<HTMLDivElement>document.querySelector('#notifyModal .modal-body')).innerHTML = `<p>${error}</p>`;
        let notifyModal = new bootstrap.Modal(document.querySelector('#notifyModal'));
        notifyModal.show();
      }
    }
    setButtonDisabled('share-screen-button', false);
    updateButtonsForPublishState();

    if (enabled) {
      (<HTMLDivElement>$(`screenshare-wrapper-${currentRoom.localParticipant.identity}`)).remove();
      calColClassScreenShare();
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

      const chatTemplate = `
      <div style="margin-bottom: 10px; text-align : right">
          <span style="font-size:12px;">${currentRoom.localParticipant.identity}</span>
          <div style="margin-top:5px">
            <span style="background:grey;color:white;padding:5px;border-radius:8px">
              ${textField.value}
            <span>
          </div>
      </div>
      `;
      (<HTMLTextAreaElement>(
        $('chat')
      )).insertAdjacentHTML("beforeend", chatTemplate);
      textField.value = '';
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
    window.location.reload();
  },

  handleScenario: (e: Event) => {
    const scenario = (<HTMLSelectElement>e.target).value;
    if (scenario !== '') {
      if (scenario === 'signal-reconnect') {
        appActions.disconnectSignal();
      } else {
        currentRoom?.simulateScenario(scenario);
      }
      (<HTMLSelectElement>e.target).value = '';
    }
  },

  disconnectSignal: () => {
    if (!currentRoom) return;
    currentRoom.engine.client.close();
    if (currentRoom.engine.client.onClose) {
      currentRoom.engine.client.onClose('manual disconnect');
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
};

declare global {
  interface Window {
    currentRoom: any;
    appActions: typeof appActions;
  }
}

window.appActions = appActions;

// --------------------------- event handlers ------------------------------- //

function handleData(msg: Uint8Array, participant?: RemoteParticipant) {
  const str = state.decoder.decode(msg);
  let from = 'server';
  if (participant) {
    from = participant.identity;
  }

  const chatTemplate = `
  <div style="margin-bottom: 10px; text-align: left">
      <span style="font-size:12px;">${from}</span>
      <div style="margin-top:5px">
        <span style="background:crimson;color:white;padding:5px;border-radius:8px">
          ${str}
        <span>
      </div>
  </div>
  `;
  (<HTMLTextAreaElement>(
    $('chat')
  )).insertAdjacentHTML("beforeend", chatTemplate);
}

function participantConnected(participant: Participant) {
  appendLog('participant', participant.identity, 'connected', participant.metadata);

  if (participant.identity == "bot") {
    participant
      .on(ParticipantEvent.TrackSubscribed, (_, pub: TrackPublication) => {
        appendLog('subscribed to track', pub.trackSid, participant.identity);
      
        const div = <HTMLDivElement>$('screenshare-area');
        const videoPub = participant.getTrack(Track.Source.Camera);
        const audioPub = participant.getTrack(Track.Source.Microphone);
        console.log("DEBUG: ", videoPub, audioPub);

        if (videoPub || audioPub) {
          div.style.display = "flex";
          div.classList.add("bg-themes");

          let screenshare = <HTMLDivElement>$(`screenshare-wrapper-${participant.identity}`);
          if (!screenshare) {
            div.insertAdjacentHTML("beforeend", `
            <div id="screenshare-wrapper-${participant.identity}">
              <div style="color: yellow; position: absolute;">
                  <span id="screenshare-info-${participant.identity}"> </span>
                  <span id="screenshare-resolution-${participant.identity}"> </span>
              </div>
              <video id="screenshare-video-${participant.identity}" autoplay playsinline></video>
              <audio id="screenshare-audio-${participant.identity}" autoplay></audio>
            </div>
            `);
            calColClassScreenShare();
          }

          const videoEnabled = videoPub && videoPub.isSubscribed && !videoPub.isMuted;
          if (videoEnabled) {
            const videoElm = <HTMLVideoElement>$(`screenshare-video-${participant.identity}`);
            videoPub.videoTrack?.attach(videoElm);
            videoElm.onresize = () => {
              updateVideoSize(videoElm, <HTMLSpanElement>$(`screenshare-resolution-${participant.identity}`));
            };
          }

          const audioEnabled = audioPub && audioPub.isSubscribed && !audioPub.isMuted;
          if (audioEnabled) {
            const audioELm = <HTMLAudioElement>$(`screenshare-audio-${participant.identity}`);
            audioPub.audioTrack?.attach(audioELm);
          }

          const infoElm = $(`screenshare-info-${participant.identity}`)!;
          infoElm.innerHTML = `Screenshare from ${participant.identity}`;
        } else {
          div.style.display = 'none';
          div.classList.remove("bg-themes");
        }
      }).on(ParticipantEvent.TrackUnsubscribed, (_, pub: TrackPublication) => {
        appendLog('unsubscribed from track', pub.trackSid);

        let screenShare = (<HTMLDivElement>$(`screenshare-wrapper-${participant.identity}`));
        if (screenShare) {
          screenShare.remove();
          calColClassScreenShare();
        }
        renderScreenShare();
      });
  } else {
    if (!participant.isCameraEnabled && !participant.isMicrophoneEnabled) {
      appendLog('connected participant empty track', participant.identity);
      renderParticipant(participant);
      renderScreenShare();
    }

    participant
      .on(ParticipantEvent.TrackSubscribed, (_, pub: TrackPublication) => {
        appendLog('subscribed to track', pub.trackSid, participant.identity);
        renderParticipant(participant);
        renderScreenShare();
      })
      .on(ParticipantEvent.TrackUnsubscribed, (_, pub: TrackPublication) => {
        appendLog('unsubscribed from track', pub.trackSid);
        renderParticipant(participant);
        renderScreenShare();

        let screenShare = (<HTMLDivElement>$(`screenshare-wrapper-${participant.identity}`));
        if (screenShare) {
          screenShare.remove();
          calColClassScreenShare();
        }
      })
      .on(ParticipantEvent.TrackMuted, (pub: TrackPublication) => {
        appendLog('track was muted', pub.trackSid, participant.identity);
        renderParticipant(participant);
      })
      .on(ParticipantEvent.TrackUnmuted, (pub: TrackPublication) => {
        appendLog('track was unmuted', pub.trackSid, participant.identity);
        renderParticipant(participant);
      })
      .on(ParticipantEvent.IsSpeakingChanged, () => {
        renderParticipant(participant);
      })
      .on(ParticipantEvent.ConnectionQualityChanged, () => {
        renderParticipant(participant);
      });
  }
}

function participantDisconnected(participant: RemoteParticipant) {
  appendLog('participant', participant.sid, 'disconnected');

  renderParticipant(participant, true);

  let screenShare = <HTMLDivElement>$(`screenshare-wrapper-${participant.identity}`);
  if (screenShare) {
    screenShare.remove();
    calColClassScreenShare();
  }
}

function handleRoomDisconnect() {
  if (!currentRoom) return;
  appendLog('disconnected from room');
  setButtonsForState(false);
  renderParticipant(currentRoom.localParticipant, true);
  currentRoom.participants.forEach((p) => {
    renderParticipant(p, true);
  });
  renderScreenShare();

  const container = $('participants-area');
  if (container) {
    container.innerHTML = '';
  }

  // clear the chat area on disconnect
  const chat = <HTMLTextAreaElement>$('chat');
  chat.innerHTML = '';

  currentRoom = undefined;
  window.currentRoom = undefined;
  window.location.reload();
}

// -------------------------- rendering helpers ----------------------------- //

function appendLog(...args: any[]) {
  // const logger = $('log')!;
  for (let i = 0; i < arguments.length; i += 1) {
    if (typeof args[i] === 'object') {
      // logger.innerHTML += `${
      //   JSON && JSON.stringify ? JSON.stringify(args[i], undefined, 2) : args[i]
      // } `;

      console.log("LOG: ", `${JSON && JSON.stringify ? JSON.stringify(args[i], undefined, 2) : args[i]
        } `);
    } else {
      // logger.innerHTML += `${args[i]} `;
      console.log("LOG: ", `${args[i]} `);
    }
  }
  // logger.innerHTML += '\n';

  // (() => {
  //   logger.scrollTop = logger.scrollHeight;
  // })();
}

// updates participant UI
function renderParticipant(participant: Participant, remove: boolean = false) {
  const container = $('participants-area');
  if (!container) return;
  const { identity } = participant;
  let div = $(`participant-${identity}`);
  if (!div && !remove) {
    div = document.createElement('div');
    div.id = `participant-${identity}`;
    //div.className = 'participant';
    div.className = 'thumb-guest me-2 mt-2';

    div.innerHTML = `
      <span id="name-${identity}" class="name">${identity}</span>
      <span class="speaking">speaking</span>
      <video id="video-${identity}" poster="${THUMB_DEFAULT}" class="video-frame"></video>
      <audio id="audio-${identity}"></audio>
      <div class="info-bar" style="display: none">
        <div style="text-align: center;">
          <span id="size-${identity}" class="size">
          </span>
          <span id="bitrate-${identity}" class="bitrate">
          </span>
        </div>
      </div>
      <div id="mic-${identity}" class="icon-mute-participant" style="display:none"><i class="fas fa-volume-mute"></i></div>
      <span class="icon-signal-participant" id="signal-${identity}"></span>
      ${participant instanceof RemoteParticipant ?
        `<div class="volume-control" style="display:none">
        <input id="volume-${identity}" type="range" min="0" max="1" step="0.1" value="1" orient="vertical" />
      </div>` : ''
      }
      
    `;
    container.appendChild(div);

    const sizeElm = $(`size-${identity}`);
    const videoElm = <HTMLVideoElement>$(`video-${identity}`);
    videoElm.onresize = () => {
      updateVideoSize(videoElm!, sizeElm!);
    };
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
    return;
  }

  // update properties
  $(`name-${identity}`)!.innerHTML = participant.identity;
  if (participant instanceof LocalParticipant) {
    $(`name-${identity}`)!.innerHTML += ' (you)';
  }
  const micElm = $(`mic-${identity}`)!;
  const signalElm = $(`signal-${identity}`)!;
  const cameraPub = participant.getTrack(Track.Source.Camera);
  const micPub = participant.getTrack(Track.Source.Microphone);
  if (participant.isSpeaking) {
    div!.classList.add('active-speaking');
  } else {
    div!.classList.remove('active-speaking');
  }

  if (participant instanceof RemoteParticipant) {
    const volumeSlider = <HTMLInputElement>$(`volume-${identity}`);
    volumeSlider.addEventListener('input', (ev) => {
      participant.setVolume(Number.parseFloat((ev.target as HTMLInputElement).value));
    });
  }

  const cameraEnabled = cameraPub && cameraPub.isSubscribed && !cameraPub.isMuted;
  if (cameraEnabled) {
    if (participant instanceof LocalParticipant) {
      // flip
      videoElm.style.transform = 'scale(-1, 1)';
    } else if (!cameraPub?.videoTrack?.attachedElements.includes(videoElm)) {
      const startTime = Date.now();
      // measure time to render
      videoElm.onloadeddata = () => {
        const elapsed = Date.now() - startTime;
        appendLog(
          `RemoteVideoTrack ${cameraPub?.trackSid} (${videoElm.videoWidth}x${videoElm.videoHeight}) rendered in ${elapsed}ms`,
        );
      };
    }
    cameraPub?.videoTrack?.attach(videoElm);
  } else {
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
      (<HTMLButtonElement>$("toggle-audio-button")).innerHTML = '<i class="fas fa-microphone"></i>';
    } else {
      // don't attach local audio
      micPub?.audioTrack?.attach(audioELm);
    }
    micElm.style.display = "none";
  } else {
    if (participant instanceof LocalParticipant) {
      (<HTMLButtonElement>$("toggle-audio-button")).innerHTML = '<i class="fas fa-microphone-slash text-danger"></i>';
    }
    micElm.style.display = "block";
  }

  switch (participant.connectionQuality) {
    case ConnectionQuality.Excellent:
      signalElm.innerHTML = '<i class="fas fa-signal text-success"></i>';
      break;
    case ConnectionQuality.Good:
      signalElm.innerHTML = '<i class="fas fa-signal-3 text-warning"></i>';
      break;
    case ConnectionQuality.Poor:
      signalElm.innerHTML = '<i class="fas fa-signal-2 text-danger"></i>';
      break;
    default:
      signalElm.innerHTML = '';
  }
}

function renderScreenShare() {
  const div = <HTMLDivElement>$('screenshare-area');
  if (!currentRoom || currentRoom.state !== RoomState.Connected) {
    div.style.display = 'none';
    div.classList.remove("bg-themes");
    return;
  }
  
  let screenSharePub = [];

  // get screen share from local
  let screenSharePubLocal: TrackPublication | undefined = currentRoom.localParticipant.getTrack(
    Track.Source.ScreenShare,
  );
  if(screenSharePubLocal) {
    screenSharePub.push({track: screenSharePubLocal, participant: currentRoom.localParticipant})
  }

  // get screen share from participants
  currentRoom.participants.forEach((p) => {
    const pub = p.getTrack(Track.Source.ScreenShare);
    if (pub?.isSubscribed) {
      screenSharePub.push({track: pub, participant: p})
    }
  });
  
  if (screenSharePub.length > 0) {
    div.style.display = "flex";
    div.classList.add("bg-themes");

    screenSharePub.forEach((screenShareInfo) => {
      let screenshare = <HTMLDivElement>$(`screenshare-wrapper-${screenShareInfo.participant.identity}`);
      if (!screenshare) {
        div.insertAdjacentHTML("beforeend", `
        <div id="screenshare-wrapper-${screenShareInfo.participant.identity}">
          <div style="color: yellow; position: absolute;">
              <span id="screenshare-info-${screenShareInfo.participant.identity}"> </span>
              <span id="screenshare-resolution-${screenShareInfo.participant.identity}"> </span>
          </div>
          <video id="screenshare-video-${screenShareInfo.participant.identity}" autoplay playsinline></video>
        </div>
        `);
        calColClassScreenShare();
        const videoElm = <HTMLVideoElement>$(`screenshare-video-${screenShareInfo.participant.identity}`);
        screenShareInfo.track.videoTrack?.attach(videoElm);
        videoElm.onresize = () => {
          updateVideoSize(videoElm, <HTMLSpanElement>$(`screenshare-resolution-${screenShareInfo.participant.identity}`));
        };
        const infoElm = $(`screenshare-info-${screenShareInfo.participant.identity}`)!;
        infoElm.innerHTML = `Screenshare from ${screenShareInfo.participant.identity}`;
      }
    });
  } else {
    if (!$("screenshare-wrapper-bot")) {
      div.style.display = 'none';
      div.classList.remove("bg-themes");
    }
  }
}

function calColClassScreenShare() {
  let screensShare = document.querySelectorAll("[id^=\"screenshare-wrapper-\"]");
  let total = screensShare.length;
  if (total > 0) {
    screensShare.forEach((el) => {
      (<HTMLDivElement>el).style.width = `${total > 1 ? "50%" : "100%"}`;
      (<HTMLDivElement>el).style.height = `${total > 1 ? "50%" : "100%"}`;
      (<HTMLDivElement>el).style.padding = `${total > 1 ? "2px" : "0"}`;
      (<HTMLDivElement>el).style.padding = `${total > 1 ? "2px" : "0"}`;
    });
  }
}

function renderBitrate() {
  if (!currentRoom || currentRoom.state !== RoomState.Connected) {
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
      populateSelect(kind, element, devices, state.defaultDevices.get(kind));
    }),
  );
}

function populateSelect(
  kind: MediaDeviceKind,
  element: HTMLSelectElement,
  devices: MediaDeviceInfo[],
  selectedDeviceId?: string,
) {
  // clear all elements
  element.innerHTML = '';
  const initialOption = document.createElement('option');
  if (kind === 'audioinput') {
    initialOption.text = 'Audio Input (default)';
  } else if (kind === 'videoinput') {
    initialOption.text = 'Video Input (default)';
  } else if (kind === 'audiooutput') {
    initialOption.text = 'Audio Output (default)';
  }
  element.appendChild(initialOption);

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
    `${lp.isCameraEnabled ? '<i class="fas fa-webcam"></i>' : '<i class="fas fa-webcam-slash text-danger"></i>'}`,
    lp.isCameraEnabled,
  );

  // audio
  setButtonState(
    'toggle-audio-button',
    `${lp.isMicrophoneEnabled ? '<i class="fas fa-microphone"></i>' : '<i class="fas fa-microphone-slash text-danger"></i>'}`,
    lp.isMicrophoneEnabled,
  );

  // screen share
  setButtonState(
    'share-screen-button',
    lp.isScreenShareEnabled ? '<i class="share-screen-slash"></i>' : '<i class="share-screen"></i>',
    lp.isScreenShareEnabled,
  );
}
