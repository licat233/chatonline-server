
-- +migrate Up

INSERT INTO `user` (uid,name,short_no,phone,zone,search_by_phone,search_by_short,new_msg_notice,voice_on,shock_on,msg_show_detail,status,is_upload_avatar,category,robot) VALUES ('_vt_visitor','访客',20001,'13100000000','0086',0,0,0,0,0,0,1,1,'system',0);
