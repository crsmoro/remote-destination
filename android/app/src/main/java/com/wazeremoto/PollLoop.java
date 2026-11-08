package com.wazeremoto;

final class PollLoop implements Runnable {
    interface Step { void run() throws Exception; }
    interface Sleep { void waitFor(long ms) throws InterruptedException; }
    interface Jitter { long next(long bound); }
    private final Step step; private final Sleep sleep; private final Jitter jitter;
    PollLoop(Step step, Sleep sleep, Jitter jitter) { this.step=step; this.sleep=sleep; this.jitter=jitter; }
    public void run() {
        long delay = 5000;
        while (!Thread.currentThread().isInterrupted()) {
            try { step.run(); delay = 5000; }
            catch (Exception e) {
                if (Thread.currentThread().isInterrupted()) break;
                try { sleep.waitFor(Math.min(60000, delay + jitter.next(Math.max(1, delay / 5)))); }
                catch (InterruptedException interrupted) { Thread.currentThread().interrupt(); break; }
                delay = Math.min(60000, delay * 2);
                continue;
            }
            try { sleep.waitFor(5000); }
            catch (InterruptedException interrupted) { Thread.currentThread().interrupt(); break; }
        }
    }
}
