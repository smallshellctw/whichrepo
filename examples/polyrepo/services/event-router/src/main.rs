use webhook_worker::WebhookEvent;

fn route_event(event: WebhookEvent) {
    println!("routing {}", event.id);
}

fn main() {}
