
##include "device.h"

// lwip netif
struct netif netif;

// IP6 address of netif
struct ipv6_addr netif_ip6addr;

// lwip TCP listener
struct tcp_pcb *listener;

// lwip TCP/IPv6 listener
struct tcp_pcb *listener_ip6;

uint8_t *device_write_buf = (uint8_t *)malloc(4096)))


void lwip_initialize (char* s_addr, char* s_netmask, char* s_ipv6_addr)
{
  goLog(BLOG_DEBUG, "lwip init");

  // NOTE: the device may fail during this, but there's no harm in not checking
  // for that at every step

  // init lwip
  lwip_init();

  // make addresses for netif
  ip_addr_t addr;
  addr.addr = (struct sockaddr_in *)resolve_ip(s_addr, 0);
  ip_addr_t netmask;
  netmask.addr = (struct sockaddr_in *)resolve_ip(s_netmask, 0);
  ip_addr_t gw;
  ip_addr_set_any(&gw);

  // init netif
  if (!netif_add(&netif, &addr, &netmask, &gw, NULL, netif_init_func, netif_input_func)) {
    goLog(BLOG_ERROR, "netif_add failed");
    return;
  }

  // set netif up
  netif_set_up(&netif);

  // set netif pretend TCP
  netif_set_pretend_tcp(&netif, 1);

  // set netif default
  netif_set_default(&netif);



  // parse IP6 address
  if (s_ipv6_addr) {
    if (!ipaddr6_parse_ipv6_addr(s_ipv6_addr, &netif_ip6addr)) {
      BLog(BLOG_ERROR, "netif ip6addr: incorrect");
      return 0;
    }
  }


  if (s_ipv6_addr) {
    // add IPv6 address
    memcpy(netif_ip6_addr(&netif, 0), netif_ip6addr.bytes, sizeof(netif_ip6addr.bytes));
    netif_ip6_addr_set_state(&netif, 0, IP6_ADDR_VALID);
  }

  // init listener
  struct tcp_pcb *l = tcp_new();
  if (!l) {
    goLog(BLOG_ERROR, "tcp_new failed");
    return;
  }

  // bind listener
  if (tcp_bind_to_netif(l, "ho0") != ERR_OK) {
    goLog(BLOG_ERROR, "tcp_bind_to_netif failed");
    tcp_close(l);
    return;
  }

  // listen listener
  if (!(listener = tcp_listen(l))) {
    goLog(BLOG_ERROR, "tcp_listen failed");
    tcp_close(l);
    return;
  }

  // setup listener accept handler
  tcp_accept(listener, listener_accept_func);

  if (s_ipv6_addr) {
    struct tcp_pcb *l_ip6 = tcp_new_ip6();
    if (!l_ip6) {
      goLog(BLOG_ERROR, "tcp_new_ip6 failed");
      return;
    }

    if (tcp_bind_to_netif(l_ip6, "ho0") != ERR_OK) {
      goLog(BLOG_ERROR, "tcp_bind_to_netif failed");
      tcp_close(l_ip6);
      return;
    }

    if (!(listener_ip6 = tcp_listen(l_ip6))) {
      goLog(BLOG_ERROR, "tcp_listen failed");
      tcp_close(l_ip6);
      return;
    }

    tcp_accept(listener_ip6, listener_accept_func);
  }

}


err_t netif_init_func (struct netif *netif)
{
  BLog(BLOG_DEBUG, "netif func init");

  netif->name[0] = 'h';
  netif->name[1] = 'o';
  netif->output = netif_output_func;
  //netif->output_ip6 = netif_output_ip6_func;

  return ERR_OK;
}

err_t netif_output_func (struct netif *netif, struct pbuf *p, ip_addr_t *ipaddr)
{
  return common_netif_output(netif, p);
}

//err_t netif_output_ip6_func (struct netif *netif, struct pbuf *p, ip6_addr_t *ipaddr);
//{
//  return common_netif_output(netif, p);
//}

err_t common_netif_output (struct netif *netif, struct pbuf *p)
{
  goLog(BLOG_DEBUG, "device write: send packet");

  if (quitting) {
    return ERR_OK;
  }

  // if there is just one chunk, send it directly, else via buffer
  if (!p->next) {
    if (p->len > BTap_GetMTU(&device)) {
      goLog(BLOG_WARNING, "netif func output: no space left");
      return ERR_OK;
    }

    goLwipOutPacket((uint8_t *)p->payload, p->len);
  } else {
    int len = 0;
    do {

      memcpy(device_write_buf + len, p->payload, p->len);
      len += p->len;
    } while (p = p->next);

    goLwipOutPacket(device_write_buf, len);
  }

  return ERR_OK;
}


err_t netif_input_func (struct pbuf *p, struct netif *inp)
{
  uint8_t ip_version = 0;
  if (p->len > 0) {
    ip_version = (((uint8_t *)p->payload)[0] >> 4);
  }

  switch (ip_version) {
    case 4: {
      return ip_input(p, inp);
    } break;
    case 6: {
        return ip6_input(p, inp);
    } break;
  }

  pbuf_free(p);
  return ERR_OK;
}

void lwip_input (uint8_t *data, int data_len)
{
  ASSERT(data_len >= 0)

  // obtain pbuf
  if (data_len > UINT16_MAX) {
    goLog(BLOG_WARNING, "device read: packet too large");
    free(data);
    return;
  }
  struct pbuf *p = pbuf_alloc(PBUF_RAW, data_len, PBUF_POOL);
  if (!p) {
    goLog(BLOG_WARNING, "device read: pbuf_alloc failed");
    free(data);
    return;
  }

  // write packet to pbuf
  ASSERT_FORCE(pbuf_take(p, data, data_len) == ERR_OK)

  // pass pbuf to input
  if (netif.input(p, &netif) != ERR_OK) {
    goLog(BLOG_WARNING, "device read: input failed");
    pbuf_free(p);
  }
}

void lwip_detrop() {

  // free listener
  if (listener_ip6) {
    tcp_close(listener_ip6);
    listener_ip6 = NULL;
  }

  if (listener) {
    tcp_close(listener);
    listener = NULL;
  }

   netif_remove(&netif);
   netif = NULL;
}


err_t listener_accept_func (void *arg, struct tcp_pcb *newpcb, err_t err)
{
  ASSERT(err == ERR_OK)

  // signal accepted
  struct tcp_pcb *this_listener = (PCB_ISIPV6(newpcb) ? listener_ip6 : listener);
  tcp_accepted(this_listener);

   // allocate client structure
  struct tcp_client *client = (struct tcp_client *)malloc(sizeof(*client));
  if (!client) {
    goLog(BLOG_ERROR, "listener accept: malloc failed");
    return ERR_MEM;
  }

  // read addresses
  local_addr = baddr_from_lwip(PCB_ISIPV6(newpcb), &newpcb->local_ip, newpcb->local_port);
  remote_addr = baddr_from_lwip(PCB_ISIPV6(newpcb), &newpcb->remote_ip, newpcb->remote_port);

  // Init Go tunnel.
  client->tunnel_id = 0;

#ifdef CGO
  if !PCB_ISIPV6(newpcb) {
      client->tunnel_id = goNewTunnel(newpcb);
      if (tunnel_id == 0) {
        goLog(BLOG_ERROR, "could not create new tunnel.");
        return ERR_MEM;
      }
  }
#endif

  // set pcb
  client->pcb = newpcb;

  // set client not closed
  client->client_closed = 0;

  // setup handler argument
  tcp_arg(client->pcb, client);

  // setup handlers
  tcp_err(client->pcb, client_err_func);
  tcp_recv(client->pcb, client_recv_func);
  tcp_sent(client->pcb, client_sent_func);

  // setup buffer
  client->buf_used = 0;

  return ERR_OK;
}


void free_tunnel_and_client(struct tcp_client *client)
{
  if (client == NULL) {
    return;
  }

  err_t err = goTunnelDestroy(client->tunnel_id);

  if (err == ERR_OK) {
    client->client_closed = 1;
    client->tunnel_id = 0;
    free(client);
  }
}


void free_connection_and_tunnel_and_client (struct tcp_client *client)
{
    ASSERT(!client->client_closed)

    // remove callbacks
    tcp_err(client->pcb, NULL);
    tcp_recv(client->pcb, NULL);
    tcp_sent(client->pcb, NULL);

    // free pcb
    err_t err = tcp_close(client->pcb);
    if (err != ERR_OK) {
      tcp_abort(client->pcb);
    }

    free_tunnel_and_client(client);
}

void client_err_func (void *arg, err_t err)
{
  struct tcp_client *client = (struct tcp_client *)arg;
  ASSERT(!client->client_closed)
  free_tunnel_and_client(client)
}

static err_t client_recv_func(void *arg, struct tcp_pcb *pcb, struct pbuf *p, err_t err)
{
  struct tcp_client *client = (struct tcp_client *)arg;

  if (client->client_closed) {
    return ERR_ABRT;
  }

  if (err != ERR_OK) {
    return ERR_ABRT;
  }

  if (!p) {
    free_connection_and_tunnel_and_client(client);
    return ERR_ABRT;
  }

  ASSERT(p->tot_len > 0)

  err_t werr;
  werr = goTunnelWrite(client->tunnel_id, p->payload, p->len);

  if (werr == ERR_OK) {
    tcp_recved(client->pcb, p->len);
  }

  pbuf_free(p);

  return werr;
}

err_t client_sent_func (void *arg, struct tcp_pcb *tpcb, u16_t len)
{
  struct tcp_client *client = (struct tcp_client *)arg;

  ASSERT(!client->client_closed)
  ASSERT(len > 0)

  if (client == NULL) {
    return ERR_ABRT;
  }

  if (client->client_closed) {
    return ERR_ABRT;
  }

  return goTunnelSentACK(client->tunnel_id, len);
}


char* resolve_ip (char *str, int noresolve)
{
    int len = strlen(str);

    char *addr_start;
    int addr_len;
    int type;

    // determine address type
    if (len >= 1 && str[0] == '[' && str[len - 1] == ']') {
        type = 6;
        addr_start = str + 1;
        addr_len = len - 2;
    } else {
        type = 4
        addr_start = str;
        addr_len = len;
    }

    // copy
    char addr_str[BADDR_MAX_ADDR_LEN + 1];
    if (addr_len > BADDR_MAX_ADDR_LEN) {
        return 0;
    }
    memcpy(addr_str, addr_start, addr_len);
    addr_str[addr_len] = '\0';

    // initialize hints
    struct addrinfo hints;
    memset(&hints, 0, sizeof(hints));
    switch (type) {
        case 6:
            hints.ai_family = AF_INET6;
            break;
        case 4:
            hints.ai_family = AF_INET;
            break;
    }
    if (noresolve) {
        hints.ai_flags |= AI_NUMERICHOST;
    }

    // call getaddrinfo
    struct addrinfo *addrs;
    int res;
    if ((res = getaddrinfo(addr_str, NULL, &hints, &addrs)) != 0) {
        return 0;
    }


    char * ret = NULL;
    // set address
    switch (type) {
        case 6:
            struct sockaddr_in *ipv6 = (struct sockaddr_in6*)malloc(sizeof(struct sockaddr_in6))

            memcpy(ipv6, ((struct sockaddr_in6 *)addrs->ai_addr)->sin6_addr.s6_addr, sizeof(addr->ipv6));
            ret = ipv6
            break;
        case 4:
            struct sockaddr_in *ipv4 = (struct sockaddr_in*)malloc(sizeof(struct sockaddr_in))
            memcpy(ipv4, ((struct sockaddr_in *)addrs->ai_addr)->sin_addr.s_addr, sizeof(addr->ipv6));
            ret = ipv4
            break;
    }

    freeaddrinfo(addrs);

    return ret;
}


