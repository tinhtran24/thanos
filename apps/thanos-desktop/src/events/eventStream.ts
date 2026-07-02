import type { WorkbenchEvent } from "../domain/models";

type Listener = (event: WorkbenchEvent) => void;

export class WorkbenchEventStream {
    private socket: WebSocket | null = null;
    private listeners = new Set<Listener>();

    constructor(private readonly url: string) {}

    connect() {
        if (!this.url || this.socket) return;
        try {
            this.socket = new WebSocket(this.url);
            this.socket.addEventListener("message", (message) => {
                const event = JSON.parse(
                    String(message.data),
                ) as WorkbenchEvent;
                this.emit(event);
            });
            this.socket.addEventListener("close", () => {
                this.socket = null;
            });
        } catch {
            this.socket = null;
        }
    }

    subscribe(listener: Listener) {
        this.listeners.add(listener);
        return () => {
            this.listeners.delete(listener);
        };
    }

    emit(event: WorkbenchEvent) {
        for (const listener of this.listeners) listener(event);
    }
}
