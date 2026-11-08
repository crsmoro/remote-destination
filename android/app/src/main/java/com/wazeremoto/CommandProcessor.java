package com.wazeremoto;

public final class CommandProcessor {
    public interface Api { Command poll() throws Exception; boolean claim(String id) throws Exception; void ack(String id, String result) throws Exception; }
    public interface Launcher { void launch(String uri) throws Exception; }
    public interface Store {
        String attempted(); String result(); boolean pending();
        void save(String id, String result, boolean pending) throws Exception;
    }
    public static final class MemoryStore implements Store {
        private String id, result; private boolean pending;
        public String attempted() { return id; } public String result() { return result; } public boolean pending() { return pending; }
        public void save(String id, String result, boolean pending) { this.id=id; this.result=result; this.pending=pending; }
    }
    public static final class Command { public final String id, uri; public Command(String id, String uri) { this.id=id; this.uri=uri; } }
    private final Store store;
    public CommandProcessor() { this(new MemoryStore()); }
    public CommandProcessor(Store store) { this.store=store; }
    public void process(Api api, Launcher launcher) throws Exception {
        if (store.attempted() != null && store.result() == null) return;
        if (store.pending()) {
            api.ack(store.attempted(), store.result());
            store.save(store.attempted(), store.result(), false);
            return;
        }
        Command command = api.poll();
        if (Thread.currentThread().isInterrupted()) throw new InterruptedException();
        if (command == null || command.id.equals(store.attempted()) || !api.claim(command.id)) return;
        store.save(command.id, null, false);
        String result;
        try { launcher.launch(command.uri); result = "intent_sent"; }
        catch (InterruptedException e) { throw e; }
        catch (Exception e) { result = "failed"; }
        store.save(command.id, result, true);
        api.ack(command.id, result);
        store.save(command.id, result, false);
    }
    public static void step(Api api, Launcher launcher) throws Exception { new CommandProcessor().process(api, launcher); }
}
