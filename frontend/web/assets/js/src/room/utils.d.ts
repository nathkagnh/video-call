import { ClientInfo } from '../proto/livekit_models';
export declare function unpackStreamId(packed: string): string[];
export declare function sleep(duration: number): Promise<void>;
/** @internal */
export declare function supportsTransceiver(): boolean;
/** @internal */
export declare function supportsAddTrack(): boolean;
export declare function supportsAdaptiveStream(): boolean;
export declare function supportsDynacast(): boolean;
export declare function supportsAV1(): boolean;
export declare function supportsSetSinkId(elm?: HTMLMediaElement): boolean;
export declare function supportsSetCodecPreferences(transceiver: RTCRtpTransceiver): boolean;
export declare function isBrowserSupported(): boolean;
export declare function isFireFox(): boolean;
export declare function isSafari(): boolean;
export declare function isMobile(): boolean;
export declare function isWeb(): boolean;
export declare function compareVersions(v1: string, v2: string): number;
export declare const getResizeObserver: () => ResizeObserver;
export declare const getIntersectionObserver: () => IntersectionObserver;
export interface ObservableMediaElement extends HTMLMediaElement {
    handleResize: (entry: ResizeObserverEntry) => void;
    handleVisibilityChanged: (entry: IntersectionObserverEntry) => void;
}
export declare function getClientInfo(): ClientInfo;
export declare function getEmptyVideoStreamTrack(): MediaStreamTrack;
export declare function getEmptyAudioStreamTrack(): MediaStreamTrack;
export declare class Future<T> {
    promise: Promise<T>;
    resolve?: (arg: T) => void;
    reject?: (e: any) => void;
    onFinally?: () => void;
    constructor(futureBase?: (resolve: (arg: T) => void, reject: (e: any) => void) => void, onFinally?: () => void);
}
//# sourceMappingURL=utils.d.ts.map