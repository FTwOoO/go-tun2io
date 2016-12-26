

#ifndef _TUN2IO_H
#define _TUN2IO_H

#include <lwip/init.h>
#include <lwip/tcp.h>
#include <lwip/priv/tcp_priv.h>
#include <lwip/netif.h>
#include <stdint.h>
#include <limits.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <netdb.h>
#include <netinet/in.h>


#define BLOG_ERROR 1
#define BLOG_WARNING 2
#define BLOG_NOTICE 3
#define BLOG_INFO 4
#define BLOG_DEBUG 5


struct tcp_client {
  struct tcp_pcb *pcb;
  int client_closed;
  uint8_t buf[TCP_WND];
  int buf_used;
  uint32_t tunnel_id;
};

void lwip_initialize (char* s_addr, char* s_netmask, char* s_ipv6_addr);
void lwip_input (uint8_t *data, int data_len);
void lwip_detrop();

err_t netif_init_func (struct netif *netif);
err_t netif_output_func (struct netif *netif, struct pbuf *p,  ip_addr_t *ipaddr);
//err_t netif_output_ip6_func (struct netif *netif, struct pbuf *p,  ip6_addr_t *ipaddr);
err_t common_netif_output (struct netif *netif, struct pbuf *p);
err_t netif_input_func (struct pbuf *p, struct netif *inp);
err_t listener_accept_func (void *arg, struct tcp_pcb *newpcb, err_t err);
void free_tunnel_and_client (struct tcp_client *client);
void free_connection_and_tunnel_and_client(struct tcp_client *client);
void client_err_func (void *arg, err_t err);
err_t client_recv_func (void *arg, struct tcp_pcb *tpcb, struct pbuf *p, err_t err);
err_t client_sent_func (void *arg, struct tcp_pcb *tpcb, u16_t len);


extern uint32_t goNewTunnel(struct tcp_pcb *client);
extern int goTunnelWrite(uint32_t tunno, char *data, size_t size);
extern int goTunnelDestroy(uint32_t tunno);
extern int goTunnelSentACK(uint32_t tunno, u16_t len);
extern void goLog(int level, char *msg);


static char charAt(char *in, int i) {
	return in[i];
}
#endif
