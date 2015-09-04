/*
 * Copyright (C) 2014 Anil Madhavapeddy <anil@recoil.org>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

#include <stdio.h>
#include <stdlib.h>
#include <errno.h>
#include <err.h>

#include <sys/types.h>
#include <sys/uio.h>
#include <dispatch/dispatch.h>
#include <vmnet/vmnet.h>
#include <pthread.h>

#include "_cgo_export.h"

void VmnetOpen(void* goIface, void* msg) {
  xpc_object_t interface_desc = xpc_dictionary_create(NULL, NULL, 0);
  xpc_dictionary_set_uint64(interface_desc, vmnet_operation_mode_key, VMNET_SHARED_MODE);

  char* uuidStr = cInterfaceGetID(goIface);

  uuid_t uuid;
  if (uuidStr != NULL && strlen(uuidStr) > 0) {
    int ok = uuid_parse(uuidStr, uuid);
    if (ok != 0) {
      uuid_generate_random(uuid);
    }
    free(uuidStr);
  } else {
    uuid_generate_random(uuid);
  }
  xpc_dictionary_set_uuid(interface_desc, vmnet_interface_id_key, uuid);

  __block interface_ref iface = NULL;
  __block void* bGoIface = goIface;

  dispatch_queue_t queue = dispatch_queue_create("xyz.natd.vmnet.start", DISPATCH_QUEUE_SERIAL);
  dispatch_semaphore_t sema = dispatch_semaphore_create(0);

  iface = vmnet_start_interface(interface_desc, queue,
    ^(vmnet_return_t status, xpc_object_t interface_param) {
      cMsgSetStatus(msg, status);

      if (status != VMNET_SUCCESS || !interface_param) {
        dispatch_semaphore_signal(sema);
        return;
      }

      unsigned int mtu = xpc_dictionary_get_uint64(interface_param, vmnet_mtu_key);
      cInterfaceSetMTU(goIface, mtu);

      unsigned int max_packet_size = xpc_dictionary_get_uint64(interface_param, vmnet_max_packet_size_key);
      cInterfaceSetMaxPacketSize(goIface, max_packet_size);

      const char *macStr = xpc_dictionary_get_string(interface_param, vmnet_mac_address_key);
      char *macStrDup = strdup(macStr);
      cInterfaceSetMAC(goIface, macStrDup);
      free(macStrDup);

      const uint8_t *idStr = xpc_dictionary_get_uuid(interface_param, vmnet_interface_id_key);
      char idStrDup[37];
      uuid_unparse_lower(idStr, &idStrDup[0]);
      cInterfaceSetID(goIface, &idStrDup[0]);

      dispatch_semaphore_signal(sema);
    });

  dispatch_semaphore_wait(sema, DISPATCH_TIME_FOREVER);
  dispatch_release(queue);

  if (iface == NULL) {
    return;
  }

  cInterfaceSetIfaceRef(goIface, iface);

  dispatch_queue_t eventq = dispatch_queue_create("xyz.natd.vmnet.events", 0);
  cInterfaceSetEventQueue(goIface, eventq);
  vmnet_interface_set_event_callback(iface, VMNET_INTERFACE_PACKETS_AVAILABLE, eventq,
    ^(interface_event_t event_id, xpc_object_t event)
    {
      unsigned int navail = xpc_dictionary_get_uint64(event, vmnet_estimated_packets_available_key);
      // printf(".... %d %d\n", event_id, navail);
      cInterfaceEmitEvent(bGoIface, event_id, navail);
    });
}

void VmnetClose(void* goIface, void* msg) {
  __block interface_ref iface = (interface_ref)cInterfaceGetIfaceRef(goIface);
  dispatch_queue_t eventq = (dispatch_queue_t)cInterfaceGetEventQueue(goIface);

  vmnet_interface_set_event_callback(iface, VMNET_INTERFACE_PACKETS_AVAILABLE, NULL, NULL);

  dispatch_queue_t queue = dispatch_queue_create("xyz.natd.vmnet.stop", DISPATCH_QUEUE_SERIAL);
  dispatch_semaphore_t sema = dispatch_semaphore_create(0);

  vmnet_return_t status = vmnet_stop_interface(iface, queue,
    ^(vmnet_return_t status) {
      cMsgSetStatus(msg, status);
      dispatch_semaphore_signal(sema);
    });

  if (status != VMNET_SUCCESS) {
    cMsgSetStatus(msg, status);
  } else {
    dispatch_semaphore_wait(sema, DISPATCH_TIME_FOREVER);
  }

  dispatch_release(queue);
  dispatch_release(eventq);
}

void VmnetRead(void* goIface, void* msg) {
  interface_ref iface = (interface_ref)cInterfaceGetIfaceRef(goIface);

  struct iovec iov;
  iov.iov_base = cMsgGetBufPtr(msg);
  iov.iov_len = cMsgGetBufLen(msg);

  struct vmpktdesc v;
  v.vm_pkt_size = iov.iov_len;
  v.vm_pkt_iov = &iov;
  v.vm_pkt_iovcnt = 1;
  v.vm_flags = 0; /* TODO no clue what this is */

  int pktcnt = 1;

  vmnet_return_t status = vmnet_read(iface, &v, &pktcnt);
  cMsgSetStatus(msg, status);

  if (status == VMNET_SUCCESS && pktcnt <= 0) {
    cMsgSetStatus(msg, 2000);
  }

  if (status == VMNET_SUCCESS && pktcnt > 0) {
    cMsgSetPacketSize(msg, v.vm_pkt_size);
    cMsgSetPacketFlags(msg, v.vm_flags);
  }
}

void VmnetWrite(void* goIface, void* msg) {
  interface_ref iface = (interface_ref)cInterfaceGetIfaceRef(goIface);

  struct iovec iov;
  iov.iov_base = cMsgGetBufPtr(msg);
  iov.iov_len = cMsgGetBufLen(msg);

  struct vmpktdesc v;
  v.vm_pkt_size = iov.iov_len;
  v.vm_pkt_iov = &iov;
  v.vm_pkt_iovcnt = 1;
  v.vm_flags = cMsgGetPacketFlags(msg);

  int pktcnt = 1;

  vmnet_return_t status = vmnet_write(iface, &v, &pktcnt);
  cMsgSetStatus(msg, status);

  if (status == VMNET_SUCCESS && pktcnt <= 0) {
    cMsgSetStatus(msg, 2001);
  }
}
