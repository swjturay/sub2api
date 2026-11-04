export interface BlueZoneCodeDemo {
  promptKey: string
  file: string
  code: string
}

export const BLUE_ZONE_CODE_DEMOS: BlueZoneCodeDemo[] = [
  {
    promptKey: 'home.blueZone.demo.prompts.debounce',
    file: 'button_debounce.c',
    code: `#define DEBOUNCE_TICKS 5

bool button_pressed(void) {
  static uint8_t stable_ticks;
  bool active = gpio_read(BUTTON_PIN) == 0;

  stable_ticks = active ? stable_ticks + 1 : 0;
  return stable_ticks == DEBOUNCE_TICKS;
}`,
  },
  {
    promptKey: 'home.blueZone.demo.prompts.heartbeat',
    file: 'heartbeat.rs',
    code: `use embedded_hal::digital::ToggleableOutputPin;

fn heartbeat_tick<P: ToggleableOutputPin>(
  led: &mut P,
  tick_ms: u32,
) {
  if tick_ms % 500 == 0 {
    let _ = led.toggle();
  }
}`,
  },
  {
    promptKey: 'home.blueZone.demo.prompts.queue',
    file: 'TaskQueue.java',
    code: `class TaskQueue {
  private final BlockingQueue<Runnable> tasks =
      new LinkedBlockingQueue<>();

  void submit(Runnable task) {
    tasks.offer(task);
  }

  void runNext() throws InterruptedException {
    tasks.take().run();
  }
}`,
  },
  {
    promptKey: 'home.blueZone.demo.prompts.backoff',
    file: 'request_with_backoff.py',
    code: `import time
import requests

def request_with_backoff(url, retries=3):
    for attempt in range(retries):
        try:
            return requests.get(url, timeout=3)
        except requests.RequestException:
            time.sleep(2 ** attempt)
    raise RuntimeError("request failed")`,
  },
]
