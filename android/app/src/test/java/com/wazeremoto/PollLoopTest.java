package com.wazeremoto;

import org.junit.Test;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import static org.junit.Assert.*;

public class PollLoopTest {
    @Test public void retriesFailuresWithBoundedBackoffAndResetsAfterSuccess() throws Exception {
        List<Long> waits = new ArrayList<>();
        int[] calls = {0};
        PollLoop loop = new PollLoop(() -> { if (++calls[0] <= 5) throw new Exception(); },
                ms -> { waits.add(ms); if (waits.size() == 6) throw new InterruptedException(); }, bound -> 0);
        loop.run();
        assertEquals(Arrays.asList(5000L, 10000L, 20000L, 40000L, 60000L, 5000L), waits);
        assertEquals(6, calls[0]);
    }
}
