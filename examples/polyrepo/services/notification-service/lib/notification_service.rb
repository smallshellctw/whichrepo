require "webhook_worker"

class NotificationService
  def deliver_webhook_alert(event)
    WebhookWorker.enqueue(event)
  end
end
